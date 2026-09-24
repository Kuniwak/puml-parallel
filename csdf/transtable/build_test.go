package transtable_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestBuild(t *testing.T) {
	type testCase struct {
		Diagram string
		Options transtable.Options
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
						{{Dst: "s1"}},
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
						{Cond: transtable.Cond{{Pred: "g"}}, Dst: "s1"},
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
						{Cond: transtable.Cond{{Pred: "g1"}}, Dst: "s1"},
						{Cond: transtable.Cond{{Pred: "g2"}}, Dst: "s2"},
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
						{Cond: transtable.Cond{{Pred: "g"}}, Dst: "s1"},
						{Dst: "s2"},
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{{Refused: true}}}},
					{State: "s2", Name: "S2", Cells: [][]transtable.Outcome{{{Refused: true}}}},
				},
			},
		},
		"an unreachable state is no row, and its events are no columns": {
			Diagram: `@startuml
state "S0" as s0
state "Z" as z
state "Y" as y
[*] --> s0
z --> y : b
y --> [*]
@enduml
`,
			Want: &transtable.Table{
				Rows:        []transtable.Row{{State: "s0", Name: "S0"}},
				Unreachable: []csdf.StateID{"y", "z"},
			},
		},
		"termination is a column after the events, filled in like one": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
[*] --> s0
s0 --> s1 : a
s1 --> [*] : g
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{{Event: "a"}, {Termination: true}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{
						{{Dst: "s1"}},
						{{Refused: true}},
					}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{
						{{Refused: true}},
						{
							{Cond: transtable.Cond{{Pred: "g"}}, Dst: transtable.Terminated},
							{Cond: transtable.Cond{{Pred: "g", Negated: true}}, Refused: true},
						},
					}},
				},
			},
		},
		"tau is not a column; what the state may do after it is in the cells of the state": {
			Diagram: `@startuml
state "Idle" as idle
state "Counting" as counting
state "Fixed" as fixed
[*] --> idle
idle --> counting : BOOK
counting --> counting : BOOK
counting --> fixed : tau
fixed --> idle : REPORT
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{{Event: "BOOK"}, {Event: "REPORT"}},
				Rows: []transtable.Row{
					{State: "idle", Name: "Idle", Cells: [][]transtable.Outcome{
						{{Dst: "counting"}},
						{{Refused: true}},
					}},
					// Counting never refuses by itself, since its tau always
					// may fire; Fixed, which the tau leads to, refuses BOOK.
					{State: "counting", Name: "Counting", Cells: [][]transtable.Outcome{
						{
							{Dst: "counting"},
							{Refused: true},
						},
						{{Dst: "idle"}},
					}},
					{State: "fixed", Name: "Fixed", Cells: [][]transtable.Outcome{
						{{Refused: true}},
						{{Dst: "idle"}},
					}},
				},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := csdf.MustParse(testCase.Diagram)

			// Act
			got, err := transtable.Build(d, testCase.Options)

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

// TestBuildFirstCell looks at one cell only, that of the start state under the
// first column, where the whole table would bury the point.
func TestBuildFirstCell(t *testing.T) {
	type testCase struct {
		Diagram string
		Options transtable.Options
		Want    []transtable.Outcome
	}

	// Two ways round a diamond that differ only in their postconditions.
	postDiamond := `@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
state "E" as E
[*] --> A
A --> B : tau ; true ; p
A --> C : tau ; true ; q
B --> D : tau
C --> D : tau
D --> E : a
@enduml
`

	testCases := map[string]testCase{
		// A tau path is one weak transition, so what it leads to is
		// conditioned on the conjunction of every guard along it; each state it
		// passes may be where the diagram stops and refuses.
		"the guards along a tau path are conjoined in order": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
[*] --> A
A --> B : tau ; h1
B --> C : tau ; h2
C --> D : a ; g
@enduml
`,
			Want: []transtable.Outcome{
				{Cond: transtable.Cond{{Pred: "h1", Negated: true}}, Refused: true},
				{Cond: transtable.Cond{{Pred: "h1"}, {Pred: "h2", Negated: true}}, Refused: true},
				{Cond: transtable.Cond{{Pred: "h1"}, {Pred: "h2"}, {Pred: "g"}}, Dst: "D"},
				{Cond: transtable.Cond{{Pred: "h1"}, {Pred: "h2"}, {Pred: "g", Negated: true}}, Refused: true},
			},
		},
		// Hiding interleaved events makes tau diamonds, and a walk that took
		// every way round each of them would grow exponentially. Two ways that
		// reach a state under the same condition and postconditions lead to
		// the same outcomes, so the second is not walked.
		"a tau diamond is walked once": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
state "E" as E
[*] --> A
A --> B : tau
A --> C : tau
B --> D : tau
C --> D : tau
D --> E : a
@enduml
`,
			Want: []transtable.Outcome{
				{Dst: "E"},
			},
		},
		// Postconditions left out would otherwise keep apart every order in
		// which hidden events that change the values interleave.
		"postconditions not asked for do not tell tau paths apart": {
			Diagram: postDiamond,
			Want: []transtable.Outcome{
				{Dst: "E"},
			},
		},
		"postconditions asked for are collected along the path, and tell tau paths apart": {
			Diagram: postDiamond,
			Options: transtable.Options{Posts: true},
			Want: []transtable.Outcome{
				{Posts: []csdf.Predicate{"p", "true", "true"}, Dst: "E"},
				{Posts: []csdf.Predicate{"q", "true", "true"}, Dst: "E"},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := csdf.MustParse(testCase.Diagram)

			// Act
			got, err := transtable.Build(d, testCase.Options)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if diff := cmp.Diff(testCase.Want, got.Rows[0].Cells[0], cmpopts.EquateEmpty()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

// A state on a tau cycle may never become stable, so it refuses nothing in the
// stable-failures sense although it accepts nothing either. A table of refusals
// cannot show that, so it is not built at all.
func TestBuildRefusesADiagramThatMayDiverge(t *testing.T) {
	// Arrange
	d := csdf.MustParse(`@startuml
state "A" as A
state "B" as B
[*] --> A
A --> B : tau
B --> A : tau ; g
@enduml
`)

	// Act
	_, err := transtable.Build(d, transtable.Options{})

	// Assert
	var livelock *transtable.LivelockError
	if !errors.As(err, &livelock) {
		t.Fatalf("want a *LivelockError, got %v", err)
	}
	if want := "A -> B -> A"; !strings.Contains(err.Error(), want) {
		t.Errorf("want the cycle %q in the message, got %q", want, err.Error())
	}
}
