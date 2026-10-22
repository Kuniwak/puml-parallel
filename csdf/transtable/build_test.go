package transtable_test

import (
	"errors"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
)

// Variables and shorthands to build conditions with, for the tests that
// spell a table.
var (
	x  = logic.Var("x")
	x1 = logic.Var("x1")
	c  = logic.Var("c")
	c1 = logic.Var("c1")

	q   = logic.Quoted
	and = logic.And
	not = logic.Not
)

func ex(vars []logic.Var, body logic.Formula) logic.Formula { return logic.Exists(vars, body) }
func vs(vars ...logic.Var) []logic.Var                      { return vars }

func goTo(cond logic.Formula, s string) transtable.Outcome {
	return transtable.Outcome{Cond: cond, Result: transtable.Goto{State: s}}
}

func refuse(cond logic.Formula) transtable.Outcome {
	return transtable.Outcome{Cond: cond, Result: transtable.Refuse{}}
}

func read(t *testing.T, src string) *core.Diagram {
	t.Helper()
	d, err := coreio.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// cells spells every cell of a table, row by row, as lines in the logical
// notation: what Build is tested for, read as a reader of the table reads it.
func cells(table *transtable.Table) map[string][]string {
	f := format(transtable.NotationLogical)
	out := map[string][]string{}
	for _, r := range table.Rows {
		for i, col := range table.Columns {
			out[r.State+" "+col.Header()] = f.Lines(r.Cells[i])
		}
	}
	return out
}

func TestBuild(t *testing.T) {
	type testCase struct {
		Diagram    string
		Conditions transtable.Conditions
		Want       map[string][]string
	}

	testCases := map[string]testCase{
		"a state without edges is a row without cells": {
			Diagram: "@startuml\nstate \"Idle\" as s0\n[*] --> s0\n@enduml\n",
			Want:    map[string][]string{},
		},
		"an edge is accepted where it starts and refused where it does not": {
			Diagram: "@startuml\nstate \"S0\" as s0\nstate \"S1\" as s1\n[*] --> s0\ns0 --> s1 : a\n@enduml\n",
			Want:    map[string][]string{"s0 a": {"→ s1"}, "s1 a": {"×"}},
		},
		"a guarded edge is accepted when its guard holds of the values and the offered field, and refused otherwise": {
			Diagram: "@startuml\nstate \"S0\" as s0\ns0 : n\nstate \"S1\" as s1\n[*] --> s0 : $n' = 0\ns0 --> s1 : a?k ; $k < $n\n@enduml\n",
			Want: map[string][]string{
				"s0 a?k": {`["$_0 < $_1"(k, n)] → s1`, `[¬"$_0 < $_1"(k, n)] ×`},
				"s1 a?k": {"×"},
			},
		},
		"edges for one event are all listed, and refused only when none of their guards holds": {
			Diagram: "@startuml\nstate \"S0\" as s0\nstate \"S1\" as s1\nstate \"S2\" as s2\n[*] --> s0\ns0 --> s1 : a ; g1\ns0 --> s2 : a ; g2\n@enduml\n",
			Want: map[string][]string{
				"s0 a": {`["g1"] → s1`, `["g2"] → s2`, `[¬"g1" ∧ ¬"g2"] ×`},
				"s1 a": {"×"}, "s2 a": {"×"},
			},
		},
		"the post of an accepted event says what the values after it are, and the full conditions keep it": {
			Diagram:    "@startuml\nstate \"S0\" as s0\ns0 : n\n[*] --> s0 : $n' = 0\ns0 --> s0 : inc ; ; $n' = $n + 1\n@enduml\n",
			Conditions: transtable.ConditionsFull,
			Want:       map[string][]string{"s0 inc": {`["$_0 = $_1 + 1"(n', n)] → s0`}},
		},
		"the enabling conditions leave out the post nothing reads": {
			Diagram:    "@startuml\nstate \"S0\" as s0\ns0 : n\n[*] --> s0 : $n' = 0\ns0 --> s0 : inc ; ; $n' = $n + 1\n@enduml\n",
			Conditions: transtable.ConditionsEnabling,
			Want:       map[string][]string{"s0 inc": {"→ s0"}},
		},
		"a tau step before the event binds the values after it, and an unstable state refuses nothing": {
			Diagram: "@startuml\nstate \"S0\" as s0\ns0 : n\nstate \"S1\" as s1\ns1 : n\nstate \"S2\" as s2\n" +
				"[*] --> s0 : $n' = 0\ns0 --> s1 : tau ; $n > 0 ; @eq(n', n)\ns1 --> s2 : a ; $n > 1\n@enduml\n",
			Want: map[string][]string{
				"s0 a": {
					`[¬"$_0 > 0"(n)] ×`,
					`[∃n@1. "$_0 > 0"(n) ∧ n@1 = n ∧ "$_0 > 1"(n@1)] → s2`,
					`[∃n@1. "$_0 > 0"(n) ∧ n@1 = n ∧ ¬"$_0 > 1"(n@1)] ×`,
				},
				"s1 a": {`["$_0 > 1"(n)] → s2`, `[¬"$_0 > 1"(n)] ×`},
				"s2 a": {"×"},
			},
		},
		"an end edge is a column of termination": {
			Diagram: "@startuml\nstate \"S0\" as s0\n[*] --> s0\ns0 --> [*] : done\n@enduml\n",
			Want:    map[string][]string{"s0 [*]": {`["done"] → [*]`, `[¬"done"] ×`}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := read(t, tc.Diagram)
			conds := tc.Conditions
			if conds == 0 {
				conds = transtable.ConditionsFull
			}

			// Execute
			table, err := transtable.Build(d, conds)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.Want, cells(table)); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestBuildRefusesADiagramItCannotTabulate(t *testing.T) {
	testCases := map[string]struct {
		Diagram string
		Want    any
	}{
		"a reachable tau cycle, which may diverge": {
			Diagram: "@startuml\nstate \"a\" as a\nstate \"b\" as b\n[*] --> a\na --> b : tau\nb --> a : tau\n@enduml\n",
			Want:    &transtable.LivelockError{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := transtable.Build(read(t, tc.Diagram), transtable.ConditionsFull)

			// Assert
			var livelock *transtable.LivelockError
			if !errors.As(err, &livelock) || len(livelock.States) != 2 {
				t.Errorf("err = %v, want the two states on the tau cycle", err)
			}
		})
	}
}

func TestBuildRefusesConditionsItDoesNotKnow(t *testing.T) {
	// Execute
	_, err := transtable.Build(read(t, "@startuml\nstate \"A\" as A\n[*] --> A\n@enduml\n"), 0)

	// Assert
	if err == nil {
		t.Error("err = nil, want the zero Conditions refused")
	}
}

func TestBuildListsTheUnreachableStates(t *testing.T) {
	// Execute
	table, err := transtable.Build(read(t, "@startuml\nstate \"S0\" as s0\nstate \"Z\" as z\n[*] --> s0\n@enduml\n"), transtable.ConditionsFull)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"z"}, table.Unreachable); diff != "" {
		t.Error(diff)
	}
}
