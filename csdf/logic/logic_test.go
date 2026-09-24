package logic_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/google/go-cmp/cmp"
)

func mustNotation(t *testing.T, c logic.Connectives) logic.Notation {
	t.Helper()
	n, err := logic.NewNotation(c)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	return n
}

var symbols = logic.Connectives{
	And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ",
	Exists: "∃", Forall: "∀", True: "true", False: "false",
}

func vs(v ...logic.Var) []logic.Var { return v }

func p(args ...logic.Var) logic.Formula { return logic.Atom("p", args...) }
func q(args ...logic.Var) logic.Formula { return logic.Atom("q", args...) }
func r(args ...logic.Var) logic.Formula { return logic.Atom("r", args...) }

func TestNotationSpell(t *testing.T) {
	type testCase struct {
		Formula logic.Formula
		Want    string
	}

	testCases := map[string]testCase{
		"an atom is its name applied to its arguments": {
			Formula: logic.Atom("guard_1", "x0"),
			Want:    "guard_1(x0)",
		},
		"an atom without arguments is its name": {
			Formula: logic.Atom("p"),
			Want:    "p",
		},
		"a quoted atom is a JSON string, so no text in it reads as a connective": {
			Formula: logic.Quoted(`a "b" ∧ <c>`, "c", "x"),
			Want:    `"a \"b\" ∧ <c>"(c, x)`,
		},
		"the constants": {
			Formula: logic.And(logic.True, logic.False),
			Want:    "true ∧ false",
		},
		"a negated atom takes no parentheses": {
			Formula: logic.Not(p("x")),
			Want:    "¬p(x)",
		},
		"a negated conjunction does": {
			Formula: logic.Not(logic.And(p("x"), q("x"))),
			Want:    "¬(p(x) ∧ q(x))",
		},
		"a conjunction inside a disjunction is parenthesised": {
			Formula: logic.Or(logic.And(p("x"), q("x")), r("x")),
			Want:    "(p(x) ∧ q(x)) ∨ r(x)",
		},
		"a disjunction inside a conjunction is parenthesised": {
			Formula: logic.And(logic.Or(p("x"), q("x")), r("x")),
			Want:    "(p(x) ∨ q(x)) ∧ r(x)",
		},
		"an implication binds loosest": {
			Formula: logic.Implies(logic.And(p("x"), q("x")), logic.Or(r("x"), p("y"))),
			Want:    "p(x) ∧ q(x) → r(x) ∨ p(y)",
		},
		"an implication inside an implication is parenthesised, either side": {
			Formula: logic.Implies(logic.Implies(p("x"), q("x")), logic.Implies(r("x"), p("y"))),
			Want:    "(p(x) → q(x)) → (r(x) → p(y))",
		},
		"a quantifier reaches to the end, so one that ends a formula takes no parentheses": {
			Formula: logic.And(p("x"), logic.Exists(vs("y"), q("x", "y"))),
			Want:    "p(x) ∧ ∃y. q(x, y)",
		},
		"one that something follows does": {
			Formula: logic.And(logic.Exists(vs("y"), q("x", "y")), p("x")),
			Want:    "(∃y. q(x, y)) ∧ p(x)",
		},
		"a negated quantifier at the end takes none": {
			Formula: logic.Exists(vs("x", "y"), logic.And(p("x"), logic.Not(logic.Exists(vs("z"), q("y", "z"))))),
			Want:    "∃x y. p(x) ∧ ¬∃z. q(y, z)",
		},
		"a negated quantifier that something follows does": {
			Formula: logic.And(logic.Not(logic.Exists(vs("z"), q("x", "z"))), p("x")),
			Want:    "¬(∃z. q(x, z)) ∧ p(x)",
		},
		"a quantifier ending a conjunction that something follows is parenthesised there": {
			Formula: logic.Or(logic.And(p("x"), logic.Exists(vs("y"), q("x", "y"))), r("x")),
			Want:    "(p(x) ∧ ∃y. q(x, y)) ∨ r(x)",
		},
		"a quantifier on the left of an implication is parenthesised": {
			Formula: logic.Forall(vs("x"), logic.Implies(logic.Exists(vs("y"), q("x", "y")), p("x"))),
			Want:    "∀x. (∃y. q(x, y)) → p(x)",
		},
		"an empty conjunction is true and an empty disjunction false": {
			Formula: logic.Implies(logic.And(), logic.Or()),
			Want:    "true → false",
		},
		"a quantifier binding nothing is its body": {
			Formula: logic.Exists(nil, p("x")),
			Want:    "p(x)",
		},
	}

	n := mustNotation(t, symbols)
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := n.Spell(testCase.Formula)

			// Assert
			if diff := cmp.Diff(testCase.Want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

// A notation spells its connectives exactly as given, spaces included.
func TestNotationSpellsItsConnectivesAsGiven(t *testing.T) {
	// Arrange
	n := mustNotation(t, logic.Connectives{
		And: " and ", Or: " or ", Not: "not ", Implies: " implies ",
		Exists: "exists ", Forall: "forall ", True: "T", False: "F",
	})
	f := logic.Forall(vs("x"), logic.Implies(logic.Or(p("x"), logic.True), logic.And(logic.Not(q("x")), logic.Exists(vs("y"), logic.False))))

	// Act
	got := n.Spell(f)

	// Assert
	if want := "forall x. p(x) or T implies not q(x) and exists y. F"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestNewNotationRefusesAConnectiveSpelledAsNothing(t *testing.T) {
	testCases := map[string]logic.Connectives{
		"no and":     {Or: " ∨ ", Not: "¬", Implies: " → ", Exists: "∃", Forall: "∀", True: "true", False: "false"},
		"no or":      {And: " ∧ ", Not: "¬", Implies: " → ", Exists: "∃", Forall: "∀", True: "true", False: "false"},
		"no not":     {And: " ∧ ", Or: " ∨ ", Implies: " → ", Exists: "∃", Forall: "∀", True: "true", False: "false"},
		"no implies": {And: " ∧ ", Or: " ∨ ", Not: "¬", Exists: "∃", Forall: "∀", True: "true", False: "false"},
		"no exists":  {And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ", Forall: "∀", True: "true", False: "false"},
		"no forall":  {And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ", Exists: "∃", True: "true", False: "false"},
		"no true":    {And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ", Exists: "∃", Forall: "∀", False: "false"},
		"no false":   {And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ", Exists: "∃", Forall: "∀", True: "true"},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := logic.NewNotation(c)

			// Assert
			if err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}

func TestValidTellsAMadeNotationFromTheZeroOne(t *testing.T) {
	if !mustNotation(t, symbols).Valid() {
		t.Error("want a notation NewNotation made to be valid")
	}
	if (logic.Notation{}).Valid() {
		t.Error("want the zero notation to be invalid")
	}
}

// The zero Notation spells every connective as nothing, which would print a
// negated formula as the formula itself; spelling with it is a programming
// error.
func TestTheZeroNotationPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want a panic, got none")
		}
	}()
	_ = logic.Notation{}.Spell(logic.Not(p("x")))
}

func TestSimplify(t *testing.T) {
	type testCase struct {
		Formula logic.Formula
		Want    logic.Formula
	}

	testCases := map[string]testCase{
		"true drops out of a conjunction": {
			Formula: logic.And(logic.True, p("x"), logic.True),
			Want:    p("x"),
		},
		"false absorbs a conjunction": {
			Formula: logic.And(p("x"), logic.False),
			Want:    logic.False,
		},
		"false drops out of a disjunction": {
			Formula: logic.Or(logic.False, p("x")),
			Want:    p("x"),
		},
		"true absorbs a disjunction": {
			Formula: logic.Or(p("x"), logic.True),
			Want:    logic.True,
		},
		"a conjunction inside a conjunction is flattened": {
			Formula: logic.And(p("x"), logic.And(q("x"), r("x"))),
			Want:    logic.And(p("x"), q("x"), r("x")),
		},
		"the negation of a constant is the other constant": {
			Formula: logic.And(logic.Not(logic.True), logic.Not(logic.False)),
			Want:    logic.False,
		},
		"a double negation cancels": {
			Formula: logic.Not(logic.Not(p("x"))),
			Want:    p("x"),
		},
		"an implication from true is its consequent": {
			Formula: logic.Implies(logic.True, p("x")),
			Want:    p("x"),
		},
		"an implication from false or to true is true": {
			Formula: logic.And(logic.Implies(logic.False, p("x")), logic.Implies(p("x"), logic.True)),
			Want:    logic.True,
		},
		"an implication to false is a negation": {
			Formula: logic.Implies(p("x"), logic.False),
			Want:    logic.Not(p("x")),
		},
		// Every domain is inhabited, so binding a variable nothing reads
		// changes nothing.
		"a variable the body does not read is unbound": {
			Formula: logic.Exists(vs("x", "y"), p("y")),
			Want:    logic.Exists(vs("y"), p("y")),
		},
		"a quantifier over a constant is the constant": {
			Formula: logic.And(logic.Exists(vs("x"), logic.True), logic.Not(logic.Forall(vs("x"), logic.False))),
			Want:    logic.True,
		},
		"a quantifier over what simplifies to a constant is the constant": {
			Formula: logic.Exists(vs("x"), logic.And(p("x"), logic.Not(logic.Exists(vs("y"), logic.True)))),
			Want:    logic.False,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := logic.Simplify(testCase.Formula)

			// Assert
			if !logic.Equal(testCase.Want, got) {
				n := mustNotation(t, symbols)
				t.Errorf("want %s, got %s", n.Spell(testCase.Want), n.Spell(got))
			}
		})
	}
}

func TestFreeVarsComeInTheOrderTheyFirstOccur(t *testing.T) {
	// Arrange
	f := logic.And(q("z", "x"), logic.Exists(vs("x"), p("x", "y")), r("z"))

	// Act
	got := logic.FreeVars(f)

	// Assert
	if diff := cmp.Diff(vs("z", "x", "y"), got); diff != "" {
		t.Error(diff)
	}
}

func TestEqual(t *testing.T) {
	type testCase struct {
		A, B logic.Formula
		Want bool
	}

	testCases := map[string]testCase{
		"the same atom":                    {A: p("x"), B: p("x"), Want: true},
		"atoms with other arguments":       {A: p("x"), B: p("y"), Want: false},
		"a quoted atom and a named one":    {A: logic.Quoted("p", "x"), B: p("x"), Want: false},
		"conjunctions in another order":    {A: logic.And(p("x"), q("x")), B: logic.And(q("x"), p("x")), Want: false},
		"the same quantified formula":      {A: logic.Exists(vs("x"), p("x")), B: logic.Exists(vs("x"), p("x")), Want: true},
		"an existential and a universal":   {A: logic.Exists(vs("x"), p("x")), B: logic.Forall(vs("x"), p("x")), Want: false},
		"the constants":                    {A: logic.True, B: logic.False, Want: false},
		"a one-conjunct conjunction is it": {A: logic.And(p("x")), B: p("x"), Want: true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := logic.Equal(testCase.A, testCase.B)

			// Assert
			if got != testCase.Want {
				t.Errorf("want %t, got %t", testCase.Want, got)
			}
		})
	}
}

func TestIsTrueAndIsFalse(t *testing.T) {
	if !logic.IsTrue(logic.True) || logic.IsTrue(p("x")) || logic.IsTrue(logic.False) {
		t.Error("want IsTrue to hold of true only")
	}
	if !logic.IsFalse(logic.False) || logic.IsFalse(p("x")) || logic.IsFalse(logic.True) {
		t.Error("want IsFalse to hold of false only")
	}
}
