package logic_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/google/go-cmp/cmp"
)

func TestAPredicateOverTerms(t *testing.T) {
	type testCase struct {
		Formula  logic.Formula
		Spelled  string
		FreeVars []logic.Var
	}

	testCases := map[string]testCase{
		"a variable is a term": {
			Formula:  logic.Pred("p", false, logic.Var("x")),
			Spelled:  "p(x)",
			FreeVars: vs("x"),
		},
		"an entry of a map, a field and an application": {
			Formula: logic.Pred("p", false,
				logic.Field(logic.Index(logic.Var("m"), logic.Var("k")), "v"),
				logic.Apply("f", logic.Var("a"), logic.Var("k"))),
			Spelled:  "p(m[k].v, f(a, k))",
			FreeVars: vs("m", "k", "a"),
		},
		"a constant is an application to nothing": {
			Formula:  logic.Pred("p", false, logic.Apply("S")),
			Spelled:  "p(S)",
			FreeVars: nil,
		},
		"a quoted predicate over terms": {
			Formula:  logic.Pred("$_ が足りる", true, logic.Index(logic.Var("在庫"), logic.Var("x'"))),
			Spelled:  `"$_ が足りる"(在庫[x'])`,
			FreeVars: vs("在庫", "x'"),
		},
		"a quantifier binds a variable inside a term": {
			Formula:  logic.Exists(vs("k"), logic.Pred("p", false, logic.Index(logic.Var("m"), logic.Var("k")))),
			Spelled:  "∃k. p(m[k])",
			FreeVars: vs("m"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			n := mustNotation(t, symbols)

			// Execute
			spelled, free := n.Spell(tc.Formula), logic.FreeVars(tc.Formula)

			// Assert
			if spelled != tc.Spelled {
				t.Errorf("want %q, got %q", tc.Spelled, spelled)
			}
			if diff := cmp.Diff(tc.FreeVars, free); diff != "" {
				t.Errorf("free vars mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAPredicateOverVariablesIsAnAtom(t *testing.T) {
	// Setup
	want := logic.Atom("p", "x", "y")

	// Execute
	got := logic.Pred("p", false, logic.Var("x"), logic.Var("y"))

	// Assert
	if !logic.Equal(want, got) {
		t.Errorf("want %#v, got %#v", want, got)
	}
}
