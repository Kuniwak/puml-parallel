package obligation_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

var symbols = func() logic.Notation {
	n, err := logic.NewNotation(logic.Connectives{
		And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ",
		Exists: "∃", Forall: "∀", True: "true", False: "false",
	})
	if err != nil {
		panic(err)
	}
	return n
}()

func TestFormulaOfAClause(t *testing.T) {
	type testCase struct {
		Clause string
		Want   string
	}

	testCases := map[string]testCase{
		"an atom is its function applied to its holes": {
			Clause: "$在庫 が $数量 以上ある",
			Want:   `"$_0 が $_1 以上ある"(在庫, 数量)`,
		},
		"a value after the step is the primed variable": {
			Clause: "$x' = $x + 1",
			Want:   `"$_0 = $_1 + 1"(x', x)`,
		},
		"a hole of a term is the term": {
			Clause: "${locations[from].在庫} > 0",
			Want:   `csdf_in(from, locations) ∧ "$_0 > 0"(locations[from].在庫)`,
		},
		"a builtin that reads an entry reads it only where it is": {
			Clause: "@ne(m[k], v)",
			Want:   "csdf_in(k, m) ∧ ¬(m[k] = v)",
		},
		"an entry under the key of another is read where both are": {
			Clause: "${m[n[k]]} > 0",
			Want:   `csdf_in(k, n) ∧ csdf_in(n[k], m) ∧ "$_0 > 0"(m[n[k]])`,
		},
		"an entry read twice is required once": {
			Clause: "${m[k]} = ${m[k]}",
			Want:   `csdf_in(k, m) ∧ "$_0 = $_0"(m[k])`,
		},
		"@eq and @ne are equalities": {
			Clause: "@eq(a, b) ∧ @ne(from, to)",
			Want:   "a = b ∧ ¬(from = to)",
		},
		"the map builtins are predicates the targets define": {
			Clause: "@in(k, m) ∧ @notin(k, n) ∧ @empty(m) ∧ @update(m', m, k, v) ∧ @frame(m', m, k)",
			Want:   "csdf_in(k, m) ∧ ¬csdf_in(k, n) ∧ csdf_empty(m) ∧ csdf_update(m', m, k, v) ∧ csdf_frame(m', m, k)",
		},
		"@at applies to the state as a constant": {
			Clause: "@at(locations[from], 倉庫待ち)",
			Want:   "csdf_in(from, locations) ∧ csdf_at(locations[from], 倉庫待ち)",
		},
		"an existential binds a fresh name of its hint": {
			Clause: "@exists(m : M) (@update(m, n, k, v) ∧ @update(n', m, j, w))",
			Want:   "∃m. csdf_update(m, n, k, v) ∧ csdf_update(n', m, j, w)",
		},
		"the fresh name avoids a free one": {
			Clause: "@exists(m : M) (@eq(m, n)) ∧ @empty(m)",
			Want:   "(∃m2. m2 = n) ∧ csdf_empty(m)",
		},
		"nothing is true": {
			Clause: "",
			Want:   "true",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			c, err := clause.Parse(tc.Clause)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := obligation.Formula(c)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if s := symbols.Spell(got); s != tc.Want {
				t.Errorf("want %q, got %q", tc.Want, s)
			}
		})
	}
}

func TestFormulaKeepsTheTypeOfAnExistential(t *testing.T) {
	// Setup
	c, err := clause.Parse("@exists(m : M) (@empty(m))")
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, err := obligation.Formula(c)

	// Assert
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if tree := logic.TreeOf(got); len(tree.Types) != 1 || tree.Types[0] != "M" {
		t.Errorf("want the type M, got %v", tree.Types)
	}
}

func TestFormulaRefusesAClauseThatIsNotWellFormed(t *testing.T) {
	// Setup
	c := clause.And{Conjuncts: []clause.Formula{clause.Atom{Parts: []clause.Part{clause.Hole{Term: clause.Bound{Index: 0}}}}}}

	// Execute
	_, err := obligation.Formula(c)

	// Assert
	if err == nil {
		t.Fatal("want an error, got nil")
	}
}
