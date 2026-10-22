package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// ValPrelude declares Val, the type of every value, as the Isabelle back end
// does (isabelle.ValPrelude): a scalar, or a map, a function from scalars to
// scalars where it is defined, so that equality of maps is extensional.
// Floats are folded into SInt for now.
const ValPrelude = `inductive Scalar where
  | SInt (i : Int)
  | SString (s : String)
  | SBool (b : Bool)
  | SArray (xs : List Scalar)
  | SDict (kvs : List (String × Scalar))
  | SRecord (state : Scalar) (fields : List (String × Scalar))
deriving Inhabited

inductive Val where
  | ValScalar (s : Scalar)
  | ValMap (f : Scalar → Option Scalar)`

// InhabitedVal lets a function into Val be declared opaque, which Lean
// allows only of an inhabited type.
const InhabitedVal = `instance : Inhabited Val := ⟨.ValScalar (.SBool false)⟩`

// mapPrelude defines what a map holds, and proves that two maps holding
// the same entries are one.
const mapPrelude = `def csdf_is_map : Val → Prop
  | .ValMap _ => True
  | _ => False

def csdf_in : Val → Val → Prop
  | .ValScalar s, .ValMap f => f s ≠ none
  | _, _ => False

def csdf_index : Val → Val → Val
  | .ValMap f, .ValScalar s =>
    match f s with
    | some v => .ValScalar v
    | none => .ValScalar (.SBool false)
  | _, _ => .ValScalar (.SBool false)

theorem csdf_map_ext : ∀ (n m : Val), csdf_is_map n → csdf_is_map m →
    (∀ (k : Val), (csdf_in k n ↔ csdf_in k m) ∧ (csdf_in k n → csdf_index n k = csdf_index m k)) → n = m
  | .ValMap f, .ValMap g, _, _, h => by
    congr 1
    funext s
    have h1 := (h (.ValScalar s)).1
    have h2 := (h (.ValScalar s)).2
    cases hf : f s <;> cases hg : g s <;> simp_all [csdf_in, csdf_index]
  | .ValScalar _, _, hn, _, _ => False.elim hn
  | .ValMap _, .ValScalar _, _, hm, _ => False.elim hm

`

const atDef = `def csdf_state : Val → Val
  | .ValScalar (.SRecord s _) => .ValScalar s
  | _ => .ValScalar (.SBool false)

def csdf_at (t s : Val) : Prop := csdf_state t = s

`

// WriteBuiltins defines the builtins b reads, as the Isabelle back end does
// (isabelle.WriteBuiltins).
func WriteBuiltins(w io.Writer, b obligationir.Builtins) error {
	if b.Maps {
		io.WriteString(w, mapPrelude)
	}
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
		fmt.Fprintf(w, "def %s : Val := .ValScalar (.SInt %d)\n\n", st, i)
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
			proof = fmt.Sprintf("fun _ h => absurd (Eq.trans (Eq.symm h.1) h.2) (by simp [%s, %s])", states[0], states[1])
		}
		fmt.Fprintf(w, "theorem %s : %s :=\n  %s\n\n", obligationir.LemmaName(i+1), f, proof)
	}
	return nil
}
