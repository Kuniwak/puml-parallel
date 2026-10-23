package target_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/lean"
)

// TestSpellFormulaParenthesisesWhatAProverWouldReadOtherwise holds both
// provers to the operands that need parentheses: a negated equality, and a
// quantifier something follows.
func TestSpellFormulaParenthesisesWhatAProverWouldReadOtherwise(t *testing.T) {
	x, y := logic.Var("x"), logic.Var("y")
	p := logic.Pred("p", true, x)
	formulas := map[string]logic.Formula{
		"¬(=)":            logic.Not(logic.Eq(x, y)),
		"∃ as an operand": logic.And(logic.ExistsTyped([]logic.Var{y}, []string{"T"}, p), p),
		"→ inside ∧":      logic.And(logic.Implies(p, p), p),
	}
	want := map[string]map[string]string{
		"isabelle": {
			"¬(=)":            `\<not> (v_x = v_y)`,
			"∃ as an operand": `(\<exists>(v_y::val). type_T v_y \<and> csdf_wf v_y \<and> (atom_p v_x)) \<and> atom_p v_x`,
			"→ inside ∧":      `(atom_p v_x \<longrightarrow> atom_p v_x) \<and> atom_p v_x`,
		},
		"lean": {
			"¬(=)":            `¬(v_x = v_y)`,
			"∃ as an operand": `(∃ (v_y : Val), type_T v_y ∧ csdf_wf v_y ∧ (atom_p v_x)) ∧ atom_p v_x`,
			"→ inside ∧":      `(atom_p v_x → atom_p v_x) ∧ atom_p v_x`,
		},
	}
	notations := map[string]obligationir.Notation{"isabelle": isabelle.Notation, "lean": lean.Notation}

	for prover, n := range notations {
		for name, f := range formulas {
			t.Run(prover+"/"+name, func(t *testing.T) {
				// Execute
				got, err := n.SpellFormula(logic.TreeOf(f))

				// Assert
				if err != nil {
					t.Fatal(err)
				}
				if got != want[prover][name] {
					t.Errorf("got %s, want %s", got, want[prover][name])
				}
			})
		}
	}
}
