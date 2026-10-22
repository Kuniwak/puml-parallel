package obligationir_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

// plain spells the connectives as words, so that a test reads what was
// built rather than a prover's notation.
var plain = obligationir.Notation{
	And: " and ", Or: " or ", Not: "not ", Implies: " implies ", True: "T", False: "F",
	Exists: func(vars []string, body string) string { return "ex " + strings.Join(vars, " ") + ". " + body },
	Forall: func(vars []string, body string) string { return "all " + strings.Join(vars, " ") + ". " + body },
}

func TestRelativize(t *testing.T) {
	type testCase struct {
		Forall bool
		Vars   []string
		Types  []string
		Want   string
		Err    string
	}

	testCases := map[string]testCase{
		"∃ conjoins the type of each variable": {
			Vars: []string{"v_k"}, Types: []string{"K"}, Want: "type_K v_k and (B)",
		},
		"∀ assumes the type of each variable": {
			Forall: true, Vars: []string{"v_j", "v_k"}, Types: []string{"J", "K"}, Want: "(type_J v_j and type_K v_k) implies (B)",
		},
		"a type variable of an axiom restricts nothing": {
			Forall: true, Vars: []string{"v_m"}, Types: []string{obligation.TypeMap}, Want: "B",
		},
		"fewer types than variables is refused": {
			Vars: []string{"v_j", "v_k"}, Types: []string{"J"}, Err: "2 variables with 1 types",
		},
		"no types at all is refused": {
			Vars: []string{"v_k"}, Err: "1 variables with 0 types",
		},
		"an empty type is refused": {
			Vars: []string{"v_k"}, Types: []string{""}, Err: "v_k has no type",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := plain.Relativize(tc.Forall, tc.Vars, tc.Types, "B")

			// Assert
			if tc.Err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.Err) {
					t.Fatalf("err = %v, want one saying %q", err, tc.Err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.Want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestSpellFormulaRefusesAQuantifierWithoutTypes(t *testing.T) {
	// Setup: an untyped ∃ would range over every value, not the clauses'.
	f := logic.TreeOf(logic.Exists([]logic.Var{"k"}, logic.Pred("p", true, logic.Var("k"))))

	// Execute
	_, err := plain.SpellFormula(f)

	// Assert
	if err == nil {
		t.Error("err = nil, want an untyped quantifier refused")
	}
}

func TestSpellFormulaRefusesATreeThatIsNoFormula(t *testing.T) {
	testCases := map[string]logic.Tree{
		"an unknown op":             {Op: "xor"},
		"a negation of two":         {Op: logic.OpNot, Operands: []logic.Tree{{Op: logic.OpTrue}, {Op: logic.OpTrue}}},
		"an implication of one":     {Op: logic.OpImplies, Operands: []logic.Tree{{Op: logic.OpTrue}}},
		"an equality of one term":   {Op: logic.OpEq, Terms: []logic.TermTree{{Op: logic.TermVar, Name: "x"}}},
		"an unknown term":           {Op: logic.OpAtom, Name: "p", Quoted: true, Terms: []logic.TermTree{{Op: "lambda"}}},
		"an index of one term":      {Op: logic.OpAtom, Name: "p", Quoted: true, Terms: []logic.TermTree{{Op: logic.TermIndex, Args: []logic.TermTree{{Op: logic.TermVar, Name: "m"}}}}},
		"a quantifier binding none": {Op: logic.OpExists, Operands: []logic.Tree{{Op: logic.OpTrue}}},
	}

	for name, tree := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := plain.SpellFormula(tree)

			// Assert
			if err == nil {
				t.Error("err = nil, want the tree refused")
			}
		})
	}
}

func TestPredNameNamesEachArityOfAFrameApart(t *testing.T) {
	testCases := map[string]struct {
		Name  string
		Arity int
		Want  string
	}{
		"a frame of no key":   {obligation.PredFrame, 2, "csdf_frame0"},
		"a frame of one key":  {obligation.PredFrame, 3, "csdf_frame1"},
		"a frame of two keys": {obligation.PredFrame, 4, "csdf_frame2"},
		"another builtin":     {obligation.PredIn, 2, "csdf_in"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := obligationir.PredName(tc.Name, tc.Arity); got != tc.Want {
				t.Errorf("PredName = %q, want %q", got, tc.Want)
			}
		})
	}
}

// TestTheIdentifiersOfTwoKindsNeverMeet spells names that look alike in
// every kind, and requires each identifier to stand for one name of one kind.
func TestTheIdentifiersOfTwoKindsNeverMeet(t *testing.T) {
	// Setup
	names := []string{"x", "state_x", "fn_x", "csdf_state_x", "atom_x", "type_x", "field_x", "v_x", "x'", "csdf_in", "csdf_index"}
	kinds := map[string]func(string) string{
		"atom":     obligationir.AtomName,
		"type":     obligationir.TypeName,
		"field":    obligationir.FieldName,
		"function": obligationir.FunctionName,
		"variable": obligationir.VarName,
		"state":    func(s string) string { return obligationir.FunctionName(obligation.StatePrefix + "." + s) },
		"family":   func(s string) string { return obligationir.FunctionName(obligation.StatePrefix + "m." + s) },
	}
	builtins := []string{obligationir.IndexSymbol, obligation.PredIn, obligation.PredEmpty, obligation.PredUpdate, obligation.PredAt, "csdf_frame1", obligationir.LemmaName(1)}
	seen := map[string]string{}
	for _, b := range builtins {
		seen[b] = "builtin " + b
	}

	// Execute and assert
	for kind, spell := range kinds {
		for _, name := range names {
			id := spell(name)
			if prev, ok := seen[id]; ok {
				t.Errorf("%s %q and %s are both %s", kind, name, prev, id)
			}
			seen[id] = kind + " " + name
		}
	}
}

func TestSymbolsRefusesOneNameOfTwoArities(t *testing.T) {
	// Setup: one atom text read with one hole and with two.
	one := logic.TreeOf(logic.Pred("p", true, logic.Var("x")))
	two := logic.TreeOf(logic.Pred("p", true, logic.Var("x"), logic.Var("y")))

	// Execute
	_, err := obligationir.Symbols(nil, one, two)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "with 1 and with 2 arguments") {
		t.Errorf("err = %v, want the two arities refused", err)
	}
}

func TestAStateConstantIsNoFunctionOfTheEnvironment(t *testing.T) {
	// Setup: a function of the environment named like a state constant.
	f := logic.TreeOf(logic.And(
		logic.Pred(obligation.PredAt, false, logic.Var("t"), logic.Apply(obligation.StatePrefix+".S")),
		logic.Pred("p", true, logic.Apply("csdf_state_S"))))

	// Execute
	syms, err := obligationir.Symbols(nil, f)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]obligationir.SymbolKind{}
	for _, s := range syms {
		kinds[s.Text] = s.Kind
	}
	if kinds[obligation.StatePrefix+".S"] != obligationir.KindState {
		t.Errorf("the state constant is %v, want a state: %v", kinds[obligation.StatePrefix+".S"], syms)
	}
	if kinds["csdf_state_S"] != obligationir.KindFunction {
		t.Errorf("csdf_state_S is %v, want a function of the environment: %v", kinds["csdf_state_S"], syms)
	}
}
