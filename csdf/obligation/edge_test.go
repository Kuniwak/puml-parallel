package obligation_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

func mustParse(t *testing.T, text string) clause.And {
	t.Helper()
	c, err := clause.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAnEdgeIsEnabledWhenSomeValuesPassItsGuardAndPost(t *testing.T) {
	type testCase struct {
		Edge func(t *testing.T) core.Edge
		Want string
	}

	testCases := map[string]testCase{
		"each received field is an argument of its own, bound with the values after": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s",
					Event: core.Event{Channel: "OUT", Fields: []core.Field{{Name: "数量"}}},
					Guard: mustParse(t, "$在庫 ≥ $数量"), Post: mustParse(t, "$在庫' = $在庫 - $数量")}
			},
			Want: `∃数量 在庫'. "$_ ≥ $_"(在庫, 数量) ∧ "$_ = $_ - $_"(在庫', 在庫, 数量)`,
		},
		"a sent field is a value the step has bound, never chosen": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s",
					Event: core.Event{Channel: "SHOW", Fields: []core.Field{{Send: true, Name: "在庫"}}},
					Post:  mustParse(t, "@eq(在庫', 在庫)")}
			},
			Want: `∃在庫'. 在庫' = 在庫`,
		},
		"an edge with nothing to choose is its guard": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "t", Event: core.Event{Channel: "GO"}, Guard: mustParse(t, "$ok")}
			},
			Want: `"$_"(ok)`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := core.Diagram{States: []string{"s", "t"}, Vars: map[string][]string{"s": {"在庫", "ok"}}, Init: "s"}
			e := tc.Edge(t)

			// Execute
			got, err := obligation.Enabled(d, e)

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

func TestAStepRelatesTheValuesBeforeTheFieldsAndTheValuesAfter(t *testing.T) {
	// Setup
	d := core.Diagram{States: []string{"s"}, Vars: map[string][]string{"s": {"在庫"}}, Init: "s"}
	e := core.Edge{Src: "s", Dst: "s",
		Event: core.Event{Channel: "IN", Fields: []core.Field{{Name: "数量"}}},
		Guard: mustParse(t, "$数量 > 0"), Post: mustParse(t, "$在庫' = $在庫 + $数量")}

	// Execute
	got, err := obligation.Step(d, e)

	// Assert
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if s, want := symbols.Spell(got), `"$_ > 0"(数量) ∧ "$_ = $_ + $_"(在庫', 在庫, 数量)`; s != want {
		t.Errorf("want %q, got %q", want, s)
	}
}

func TestAnEdgeOfAnUndeclaredStateIsRefused(t *testing.T) {
	// Setup
	d := core.Diagram{States: []string{"s"}, Vars: map[string][]string{}, Init: "s"}
	e := core.Edge{Src: "s", Dst: "nowhere", Event: core.Event{Channel: "GO"}}

	// Execute
	_, err := obligation.Enabled(d, e)

	// Assert
	if err == nil {
		t.Fatal("want an error, got nil")
	}
}
