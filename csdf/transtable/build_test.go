package transtable_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func parse(t *testing.T, src string) *csdf.Diagram {
	t.Helper()
	d, err := csdf.Parse(src)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	return d
}

func TestBuild(t *testing.T) {
	type testCase struct {
		Diagram string
		Want    *transtable.Table
	}

	testCases := map[string]testCase{
		"a state without edges is a row without cells": {
			Diagram: `@startuml
state "Idle" as s0
[*] --> s0
@enduml
`,
			Want: &transtable.Table{
				Rows: []transtable.Row{{State: "s0", Name: "Idle"}},
			},
		},
		"an edge is accepted where it starts and refused where it does not": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{
						{{Posts: []csdf.Predicate{"true"}, Dst: "s1"}},
					}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{
						{{Refused: true}},
					}},
				},
			},
		},
		"a guarded edge is accepted when its guard holds and refused otherwise": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
[*] --> s0
s0 --> s1 : a ; g
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{
						{Cond: transtable.Cond{{Pred: "g"}}, Posts: []csdf.Predicate{"true"}, Dst: "s1"},
						{Cond: transtable.Cond{{Pred: "g", Negated: true}}, Refused: true},
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{
						{{Refused: true}},
					}},
				},
			},
		},
		"edges for one event are all listed, and refused only when none of their guards holds": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
state "S2" as s2
[*] --> s0
s0 --> s1 : a ; g1
s0 --> s2 : a ; g2
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{
						{Cond: transtable.Cond{{Pred: "g1"}}, Posts: []csdf.Predicate{"true"}, Dst: "s1"},
						{Cond: transtable.Cond{{Pred: "g2"}}, Posts: []csdf.Predicate{"true"}, Dst: "s2"},
						{Cond: transtable.Cond{{Pred: "g1", Negated: true}, {Pred: "g2", Negated: true}}, Refused: true},
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{{Refused: true}}}},
					{State: "s2", Name: "S2", Cells: [][]transtable.Outcome{{{Refused: true}}}},
				},
			},
		},
		"an unconditional edge for an event means the event is never refused there": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
state "S2" as s2
[*] --> s0
s0 --> s1 : a ; g
s0 --> s2 : a
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{
						{Cond: transtable.Cond{{Pred: "g"}}, Posts: []csdf.Predicate{"true"}, Dst: "s1"},
						{Posts: []csdf.Predicate{"true"}, Dst: "s2"},
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{{Refused: true}}}},
					{State: "s2", Name: "S2", Cells: [][]transtable.Outcome{{{Refused: true}}}},
				},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := parse(t, testCase.Diagram)

			// Act
			got, err := transtable.Build(d)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if diff := cmp.Diff(testCase.Want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
