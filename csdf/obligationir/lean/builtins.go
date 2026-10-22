package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// mapPrelude defines a map: a ValMap, the entry of a key the first entry of
// the list holding it.
const mapPrelude = `-- A map is a ValMap; the entry of a key is the first one holding it.
open Classical in
noncomputable def csdf_lookup : List (Val × Val) → Val → Val
  | [], _ => .ValBool false
  | (j, v) :: kvs, k => if j = k then v else csdf_lookup kvs k

def csdf_is_map (m : Val) : Prop :=
  match m with
  | .ValMap _ => True
  | _ => False

def csdf_in (k m : Val) : Prop :=
  match m with
  | .ValMap kvs => k ∈ kvs.map Prod.fst
  | _ => False

noncomputable def csdf_index (m k : Val) : Val :=
  match m with
  | .ValMap kvs => csdf_lookup kvs k
  | _ => .ValBool false

`

const atDef = `-- TODO(csdf): not formalised; the state an instance is at is uninterpreted.
opaque csdf_state : Val → Val

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
			proof = fmt.Sprintf("fun _ h => absurd (Eq.trans (Eq.symm h.1) h.2) (by simp [%s, %s])", states[0], states[1])
		}
		fmt.Fprintf(w, "theorem %s : %s :=\n  %s\n\n", obligationir.LemmaName(i+1), f, proof)
	}
	return nil
}
