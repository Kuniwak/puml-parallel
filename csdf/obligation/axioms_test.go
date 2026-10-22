package obligation_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
	"github.com/google/go-cmp/cmp"
)

func TestTheAxiomsOfTheSymbolsClausesUse(t *testing.T) {
	type testCase struct {
		Clauses []string
		Want    []string
	}

	testCases := map[string]testCase{
		"no builtin needs no axiom": {
			Clauses: []string{"$x > 0", "@eq(a, b)"},
			Want:    nil,
		},
		"an empty map has no key": {
			Clauses: []string{"@empty(m)"},
			Want:    []string{"∀m k. csdf_empty(m) → ¬csdf_in(k, m)"},
		},
		"an update maps its key and keeps every other entry": {
			Clauses: []string{"@update(m', m, k, v)"},
			Want: []string{"∀n m k v. csdf_update(n, m, k, v) → csdf_in(k, n) ∧ n[k] = v ∧ " +
				"∀j. ¬(j = k) → (csdf_in(j, n) → csdf_in(j, m)) ∧ (csdf_in(j, m) → csdf_in(j, n) ∧ n[j] = m[j])"},
		},
		"a frame keeps every entry but at its keys, one axiom for each number of keys": {
			Clauses: []string{"@frame(n', n, a)", "@frame(m', m, a, b) ∧ @frame(o', o, c)"},
			Want: []string{
				"∀n m k1. csdf_frame(n, m, k1) → ∀j. ¬(j = k1) → (csdf_in(j, n) → csdf_in(j, m)) ∧ (csdf_in(j, m) → csdf_in(j, n) ∧ n[j] = m[j])",
				"∀n m k1 k2. csdf_frame(n, m, k1, k2) → ∀j. ¬(j = k1) ∧ ¬(j = k2) → (csdf_in(j, n) → csdf_in(j, m)) ∧ (csdf_in(j, m) → csdf_in(j, n) ∧ n[j] = m[j])",
			},
		},
		"an instance is at one state at a time": {
			Clauses: []string{"@at(x, A)", "@exists(y : Y) (@at(y, B) ∧ @at(y, A))"},
			Want:    []string{"∀t. ¬(csdf_at(t, csdf_state_A) ∧ csdf_at(t, csdf_state_B))"},
		},
		"states of different families are never paired": {
			Clauses: []string{"@at(m[x], A) ∧ @at(m[y], B) ∧ @at(n[z], A)"},
			Want:    []string{"∀t. ¬(csdf_at(t, csdf_state_m.A) ∧ csdf_at(t, csdf_state_m.B))"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			var cs []clause.Formula
			for _, text := range tc.Clauses {
				cs = append(cs, mustParse(t, text))
			}

			// Execute
			got := obligation.Axioms(cs...)

			// Assert
			var spelled []string
			for _, a := range got {
				spelled = append(spelled, symbols.Spell(a))
			}
			if diff := cmp.Diff(tc.Want, spelled); diff != "" {
				t.Errorf("axioms mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEveryVariableOfAnAxiomHasAType(t *testing.T) {
	type testCase struct {
		Clause string
		Want   [][]string
	}

	testCases := map[string]testCase{
		"an empty map ranges over maps and keys": {
			Clause: "@empty(m)",
			Want:   [][]string{{"'map", "'key"}},
		},
		"an update ranges over maps, keys and values": {
			Clause: "@update(m', m, k, v)",
			Want:   [][]string{{"'map", "'map", "'key", "'value"}, {"'key"}},
		},
		"a frame ranges over maps and its keys": {
			Clause: "@frame(m', m, a)",
			Want:   [][]string{{"'map", "'map", "'key"}, {"'key"}},
		},
		"an instance is a value of its map": {
			Clause: "@at(m[x], A) ∧ @at(m[y], B)",
			Want:   [][]string{{"'value"}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			c := mustParse(t, tc.Clause)

			// Execute
			got := obligation.Axioms(c)

			// Assert
			var types [][]string
			for _, a := range got {
				types = append(types, quantifiedTypes(logic.TreeOf(a))...)
			}
			if diff := cmp.Diff(tc.Want, types); diff != "" {
				t.Errorf("types mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// quantifiedTypes are the types of every quantifier in tree, outermost first.
func quantifiedTypes(tree logic.Tree) [][]string {
	var out [][]string
	if tree.Op == logic.OpForall || tree.Op == logic.OpExists {
		out = append(out, tree.Types)
	}
	for _, o := range tree.Operands {
		out = append(out, quantifiedTypes(o)...)
	}
	return out
}
