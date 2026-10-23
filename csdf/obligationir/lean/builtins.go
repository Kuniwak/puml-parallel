package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// ValPrelude declares Val, the type of every value, as the Isabelle back end
// does (isabelle.ValPrelude). Floats are folded into ValInt for now.
const ValPrelude = `inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
  | ValRecord (state : Val) (fields : List (String × Val))
  | ValMap (kvs : List (Val × Val))`

// InhabitedVal lets a function into Val be declared opaque, which Lean
// allows only of an inhabited type.
const InhabitedVal = `instance : Inhabited Val := ⟨.ValInt 0⟩`

// mapPrelude says what a map is - a list of its entries, of distinct keys,
// the one list of its entries csdf_canon chooses - and what it holds, and
// proves that two maps holding the same entries are one (csdf_map_ext).
const mapPrelude = `-- A map is the list of its entries csdf_canon chooses among the lists of them,
-- so that two maps holding the same entries are one.
def csdf_same (a b : List (Val × Val)) : Prop := ∀ p, p ∈ a ↔ p ∈ b

noncomputable def csdf_canon (kvs : List (Val × Val)) : List (Val × Val) :=
  Classical.epsilon (fun l => csdf_same l kvs)

-- A value is well formed when every map in it is canonical, of distinct keys.
inductive csdf_wf : Val → Prop where
  | int (i : Int) : csdf_wf (.ValInt i)
  | string (s : String) : csdf_wf (.ValString s)
  | bool (b : Bool) : csdf_wf (.ValBool b)
  | array (xs : List Val) : (∀ x ∈ xs, csdf_wf x) → csdf_wf (.ValArray xs)
  | dict (kvs : List (String × Val)) : (∀ p ∈ kvs, csdf_wf p.2) → csdf_wf (.ValDict kvs)
  | record (st : Val) (fs : List (String × Val)) : csdf_wf st → (∀ p ∈ fs, csdf_wf p.2) →
      csdf_wf (.ValRecord st fs)
  | map (kvs : List (Val × Val)) : (kvs.map Prod.fst).Nodup → kvs = csdf_canon kvs →
      (∀ p ∈ kvs, csdf_wf p.1) → (∀ p ∈ kvs, csdf_wf p.2) → csdf_wf (.ValMap kvs)

def csdf_is_map : Val → Prop
  | .ValMap _ => True
  | _ => False

def csdf_in (k m : Val) : Prop :=
  match m with
  | .ValMap kvs => ∃ v, (k, v) ∈ kvs
  | _ => False

open Classical in
noncomputable def csdf_index (m k : Val) : Val :=
  match m with
  | .ValMap kvs => if h : ∃ v, (k, v) ∈ kvs then Classical.choose h else .ValBool false
  | _ => .ValBool false

theorem csdf_unique : ∀ (kvs : List (Val × Val)), (kvs.map Prod.fst).Nodup →
    ∀ k v w, (k, v) ∈ kvs → (k, w) ∈ kvs → v = w
  | [], _, _, _, _, hv, _ => by simp at hv
  | (j, x) :: rest, hn, k, v, w, hv, hw => by
    simp only [List.map_cons, List.nodup_cons, List.mem_map] at hn
    simp only [List.mem_cons, Prod.mk.injEq] at hv hw
    rcases hv with ⟨rfl, rfl⟩ | hv <;> rcases hw with ⟨hk, rfl⟩ | hw
    · rfl
    · exact absurd ⟨(k, w), hw, rfl⟩ hn.1
    · subst hk; exact absurd ⟨(k, v), hv, rfl⟩ hn.1
    · exact csdf_unique rest hn.2 k v w hv hw

theorem csdf_canon_eq (a b : List (Val × Val)) (h : csdf_same a b) : csdf_canon a = csdf_canon b := by
  unfold csdf_canon
  congr 1
  funext l
  apply propext
  exact ⟨fun hl p => (hl p).trans (h p), fun hl p => (hl p).trans (h p).symm⟩

theorem csdf_map_half : ∀ (a b : List (Val × Val)), (a.map Prod.fst).Nodup →
    (∀ k, (csdf_in k (.ValMap a) ↔ csdf_in k (.ValMap b)) ∧
      (csdf_in k (.ValMap a) → csdf_index (.ValMap a) k = csdf_index (.ValMap b) k)) →
    ∀ p, p ∈ a → p ∈ b := by
  intro a b na h p hp
  obtain ⟨k, v⟩ := p
  have ina : ∃ v, (k, v) ∈ a := ⟨v, hp⟩
  have inb : ∃ v, (k, v) ∈ b := (h k).1.mp ina
  have he := (h k).2 ina
  simp only [csdf_index, dif_pos ina, dif_pos inb] at he
  have v1 : Classical.choose ina = v := csdf_unique a na k _ _ (Classical.choose_spec ina) hp
  rw [← v1, he]
  exact Classical.choose_spec inb

-- Two well-formed maps holding the same entries are one.
theorem csdf_map_ext : ∀ (n m : Val), csdf_wf n → csdf_wf m → csdf_is_map n → csdf_is_map m →
    (∀ (k : Val), (csdf_in k n ↔ csdf_in k m) ∧ (csdf_in k n → csdf_index n k = csdf_index m k)) → n = m := by
  intro n m wn wm hn hm h
  cases wn <;> cases wm <;> simp only [csdf_is_map] at hn hm
  case map.map a na ca _ _ b nb cb _ _ =>
    have same : csdf_same a b := fun p =>
      ⟨csdf_map_half a b na h p,
       csdf_map_half b a nb (fun k => ⟨(h k).1.symm, fun i => ((h k).2 ((h k).1.mpr i)).symm⟩) p⟩
    rw [ca, cb, csdf_canon_eq a b same]

`

// atDef defines that an instance, a record, is at a state: its state is it.
// A value that is no record is at no state.
const atDef = `def csdf_at : Val → Val → Prop
  | .ValRecord st _, s => st = s
  | _, _ => False

`

// WriteBuiltins defines the builtins b reads, as the Isabelle back end does
// (isabelle.WriteBuiltins). What a map is comes first in every obligation,
// for a quantifier over a type is restricted to well-formed values.
func WriteBuiltins(w io.Writer, b obligationir.Builtins) error {
	io.WriteString(w, mapPrelude)
	defs, err := obligationir.BuiltinDefs(Notation, b)
	if err != nil {
		return fmt.Errorf("lean.WriteBuiltins: %w", err)
	}
	for _, d := range defs {
		fmt.Fprintf(w, "def %s (%s : Val) : Prop :=\n  %s\n\n", d.Name, strings.Join(d.Params, " "), d.Body)
	}
	if b.At {
		io.WriteString(w, atDef)
	}
	for i, st := range b.States {
		fmt.Fprintf(w, "def %s : Val := .ValInt %d\n\n", st, i)
	}
	return nil
}

// WriteLemmas proves what the builtins mean from their definitions, by the
// projection of a definition the lemma's shape says, so that the prover
// checks it rather than an obligation assuming it.
func WriteLemmas(w io.Writer, lemmas []logic.Tree) error {
	for i, l := range lemmas {
		f, err := Notation.SpellFormula(l)
		if err != nil {
			return fmt.Errorf("lean.WriteLemmas: %w", err)
		}
		kind, n, states, err := obligationir.LemmaShape(l)
		if err != nil {
			return fmt.Errorf("lean.WriteLemmas: %w", err)
		}
		var proof string
		switch kind {
		case obligationir.LemmaEmpty:
			proof = "fun _ k h => And.right h k"
		case obligationir.LemmaMeaning:
			proof = "fun" + strings.Repeat(" _", n) + " h => And.right (And.right h)"
		case obligationir.LemmaApart:
			proof = fmt.Sprintf("by\n    intro t h\n    cases t <;> simp_all [csdf_at, %s, %s]\n    all_goals (obtain ⟨rfl, h2⟩ := h; simp at h2)", states[0], states[1])
		}
		fmt.Fprintf(w, "theorem %s : %s :=\n  %s\n\n", obligationir.LemmaName(i+1), f, proof)
	}
	return nil
}
