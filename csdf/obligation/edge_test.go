package obligation_test

import (
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/logic"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligation"
	"github.com/google/go-cmp/cmp"
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
			Want: `∃数量 在庫'. "$_0 ≥ $_1"(在庫, 数量) ∧ "$_0 = $_1 - $_2"(在庫', 在庫, 数量)`,
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
			Want: `"$_0"(ok)`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			n := types.Named("個数")
			d := core.Diagram{States: []string{"s", "t"}, Vars: map[string][]string{"s": {"在庫", "ok"}}, Init: "s",
				Types: map[string]types.Type{"在庫": n, "ok": types.Named("真偽")}, Channels: map[string][]types.Type{"OUT": {n}, "SHOW": {n}}}
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
	type testCase struct {
		Edge func(t *testing.T) core.Edge
		Want string
	}

	testCases := map[string]testCase{
		"its guard and its post, over the fields it receives": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s",
					Event: core.Event{Channel: "IN", Fields: []core.Field{{Name: "数量"}}},
					Guard: mustParse(t, "$数量 > 0"), Post: mustParse(t, "$在庫' = $在庫 + $数量")}
			},
			Want: `"$_0 > 0"(数量) ∧ "$_0 = $_1 + $_2"(在庫', 在庫, 数量)`,
		},
		"a sent field is the value it sends": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s",
					Event: core.Event{Channel: "SHOW", Fields: []core.Field{{Send: true, Name: "在庫"}}},
					Post:  mustParse(t, "@eq(在庫', 在庫)")}
			},
			Want: `在庫' = 在庫`,
		},
		"an edge without a guard is its post": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "t", Event: core.Event{Channel: "GO"}, Post: mustParse(t, "$在庫 > 0")}
			},
			Want: `"$_0 > 0"(在庫)`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := core.Diagram{States: []string{"s", "t"}, Vars: map[string][]string{"s": {"在庫"}}, Init: "s"}
			e := tc.Edge(t)

			// Execute
			got, err := obligation.Step(d, e)

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

func TestAnEdgeThatReadsWhatItDoesNotBindIsRefused(t *testing.T) {
	type testCase struct {
		Edge func(t *testing.T) core.Edge
		Want string
	}

	testCases := map[string]testCase{
		"an undeclared source": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "nowhere", Dst: "s", Event: core.Event{Channel: "GO"}}
			},
			Want: "no state nowhere",
		},
		"an undeclared destination": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "nowhere", Event: core.Event{Channel: "GO"}}
			},
			Want: "no state nowhere",
		},
		"a guard reading a name nothing binds": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s", Event: core.Event{Channel: "GO"}, Guard: mustParse(t, "$水位 > 0")}
			},
			Want: "水位",
		},
		"a guard reading a value after the step": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s", Event: core.Event{Channel: "GO"}, Guard: mustParse(t, "$在庫' > 0")}
			},
			Want: "after the step",
		},
		"a post giving a value its destination does not hold": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "t", Event: core.Event{Channel: "GO"}, Post: mustParse(t, "$在庫' > 0")}
			},
			Want: "在庫'",
		},
		"a sent field nothing binds": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s", Event: core.Event{Channel: "OUT", Fields: []core.Field{{Send: true, Name: "額"}}}}
			},
			Want: "sends 額",
		},
		"a field received twice": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "s", Event: core.Event{Channel: "IN", Fields: []core.Field{{Name: "額"}, {Name: "額"}}}}
			},
			Want: "receives 額 twice",
		},
		"a received field named like a state variable of the source": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "s", Dst: "t", Event: core.Event{Channel: "IN", Fields: []core.Field{{Name: "在庫"}}}}
			},
			Want: "在庫",
		},
		"a received field named like a state variable of the destination": {
			Edge: func(t *testing.T) core.Edge {
				return core.Edge{Src: "t", Dst: "s", Event: core.Event{Channel: "IN", Fields: []core.Field{{Name: "在庫"}}}}
			},
			Want: "在庫",
		},
	}

	for name, tc := range testCases {
		for _, op := range []struct {
			name string
			f    func(core.Diagram, core.Edge) (logic.Formula, error)
		}{{"Step", obligation.Step}, {"Enabled", obligation.Enabled}} {
			t.Run(op.name+"/"+name, func(t *testing.T) {
				// Setup
				d := core.Diagram{States: []string{"s", "t"}, Vars: map[string][]string{"s": {"在庫"}}, Init: "s"}
				e := tc.Edge(t)

				// Execute
				_, err := op.f(d, e)

				// Assert
				if err == nil {
					t.Fatal("want an error, got nil")
				}
				if !strings.Contains(err.Error(), tc.Want) {
					t.Errorf("want an error saying %q, got %v", tc.Want, err)
				}
			})
		}
	}
}

func TestEnabledTypesWhatItChoosesByItsDeclaredType(t *testing.T) {
	// Setup
	d := core.Diagram{States: []string{"s"}, Vars: map[string][]string{"s": {"在庫"}}, Init: "s",
		Types:    map[string]types.Type{"在庫": types.Named("個数")},
		Channels: map[string][]types.Type{"OUT": {types.Named("要求量")}}}
	e := core.Edge{Src: "s", Dst: "s",
		Event: core.Event{Channel: "OUT", Fields: []core.Field{{Name: "数量"}}},
		Guard: mustParse(t, "$在庫 ≥ $数量"), Post: mustParse(t, "$在庫' = $在庫 - $数量")}
	want := []string{"要求量", "個数"}

	// Execute
	got, err := obligation.Enabled(d, e)

	// Assert
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if diff := cmp.Diff(want, logic.TreeOf(got).Types); diff != "" {
		t.Errorf("types mismatch (-want +got):\n%s", diff)
	}
}
