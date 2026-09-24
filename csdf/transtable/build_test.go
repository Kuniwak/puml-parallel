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

// spelled is a table with every outcome written as its line in the logical
// notation, which says in a line what a struct literal says in five.
type spelled struct {
	Columns     []transtable.Column
	Rows        []spelledRow
	Unreachable []csdf.StateID
}

type spelledRow struct {
	State csdf.StateID
	Name  string
	Cells [][]string
}

func spell(t *transtable.Table) *spelled {
	rows := make([]spelledRow, 0, len(t.Rows))
	for _, row := range t.Rows {
		cells := make([][]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, spellCell(cell))
		}
		rows = append(rows, spelledRow{State: row.State, Name: row.Name, Cells: cells})
	}
	return &spelled{Columns: t.Columns, Rows: rows, Unreachable: t.Unreachable}
}

func spellCell(os []transtable.Outcome) []string {
	lines := make([]string, 0, len(os))
	for _, o := range os {
		lines = append(lines, transtable.Format{Notation: transtable.NotationLogical}.Line(o))
	}
	return lines
}

func TestBuild(t *testing.T) {
	type testCase struct {
		Diagram string
		Options transtable.Options
		Want    *spelled
	}

	testCases := map[string]testCase{
		"a state without edges is a row without cells": {
			Diagram: `@startuml
state "Idle" as s0
[*] --> s0
@enduml
`,
			Want: &spelled{
				Rows: []spelledRow{{State: "s0", Name: "Idle"}},
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
			Want: &spelled{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []spelledRow{
					{State: "s0", Name: "S0", Cells: [][]string{
						{"→ s1"},
					}},
					{State: "s1", Name: "S1", Cells: [][]string{
						{"×"},
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
			Want: &spelled{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []spelledRow{
					{State: "s0", Name: "S0", Cells: [][]string{
						{"[g] → s1", "[¬(g)] ×"},
					}},
					{State: "s1", Name: "S1", Cells: [][]string{
						{"×"},
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
			Want: &spelled{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []spelledRow{
					{State: "s0", Name: "S0", Cells: [][]string{
						{"[g1] → s1", "[g2] → s2", "[¬(g1) ∧ ¬(g2)] ×"},
					}},
					{State: "s1", Name: "S1", Cells: [][]string{{"×"}}},
					{State: "s2", Name: "S2", Cells: [][]string{{"×"}}},
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
			Want: &spelled{
				Columns: []transtable.Column{{Event: "a"}},
				Rows: []spelledRow{
					{State: "s0", Name: "S0", Cells: [][]string{
						{"[g] → s1", "→ s2"},
					}},
					{State: "s1", Name: "S1", Cells: [][]string{{"×"}}},
					{State: "s2", Name: "S2", Cells: [][]string{{"×"}}},
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
			Want: &spelled{
				Rows:        []spelledRow{{State: "s0", Name: "S0"}},
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
			Want: &spelled{
				Columns: []transtable.Column{{Event: "a"}, {Termination: true}},
				Rows: []spelledRow{
					{State: "s0", Name: "S0", Cells: [][]string{
						{"→ s1"},
						{"×"},
					}},
					{State: "s1", Name: "S1", Cells: [][]string{
						{"×"},
						{"[g] → [*]", "[¬(g)] ×"},
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
			Want: &spelled{
				Columns: []transtable.Column{{Event: "BOOK"}, {Event: "REPORT"}},
				Rows: []spelledRow{
					{State: "idle", Name: "Idle", Cells: [][]string{
						{"→ counting"},
						{"×"},
					}},
					// Counting never refuses by itself, since its tau always
					// may fire; Fixed, which the tau leads to, refuses BOOK.
					{State: "counting", Name: "Counting", Cells: [][]string{
						{"→ counting", "×"},
						{"→ idle"},
					}},
					{State: "fixed", Name: "Fixed", Cells: [][]string{
						{"×"},
						{"→ idle"},
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
			if diff := cmp.Diff(testCase.Want, spell(got), cmpopts.EquateEmpty()); diff != "" {
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
		Want    []string
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
			Want: []string{
				"[¬(h1)] ×",
				"[h1 ∧ ¬(h2)] ×",
				"[h1 ∧ h2 ∧ g] → D",
				"[h1 ∧ h2 ∧ ¬(g)] ×",
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
			Want: []string{
				"→ E",
			},
		},
		"tau paths that meet again under different conditions are both walked": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
state "E" as E
[*] --> A
A --> B : tau ; g1
A --> C : tau ; g2
B --> D : tau
C --> D : tau
D --> E : a
@enduml
`,
			Want: []string{
				"[¬(g1) ∧ ¬(g2)] ×",
				"[g1] → E",
				"[g2] → E",
			},
		},
		// A refusal needs the state stable and unable to take the event, and
		// its condition says so in that order.
		"a refusal negates the tau guards, then the guards for the event": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : a ; g
A --> C : tau ; h
@enduml
`,
			Want: []string{
				"[g] → B",
				"[¬(h) ∧ ¬(g)] ×",
				"[h] ×",
			},
		},
		// The path to D is long enough for its condition to have spare
		// capacity, so the two tau edges out of D would write their guards
		// over each other if a branch extended the condition in place.
		"sibling tau edges keep their own conditions": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
state "E1" as E1
state "E2" as E2
state "F" as F
[*] --> A
A --> B : tau ; h1
B --> C : tau ; h2
C --> D : tau ; h3
D --> E1 : tau ; k1
D --> E2 : tau ; k2
E1 --> F : a
E2 --> F : a
@enduml
`,
			Want: []string{
				"[¬(h1)] ×",
				"[h1 ∧ ¬(h2)] ×",
				"[h1 ∧ h2 ∧ ¬(h3)] ×",
				"[h1 ∧ h2 ∧ h3 ∧ ¬(k1) ∧ ¬(k2)] ×",
				"[h1 ∧ h2 ∧ h3 ∧ k1] → F",
				"[h1 ∧ h2 ∧ h3 ∧ k2] → F",
			},
		},
		// The grammar lets a guard hold any character but a semicolon, so one
		// guard must never be taken for two.
		"a guard holding a control character is not taken for two guards": {
			Diagram: "@startuml\n" +
				"state \"A\" as A\nstate \"B\" as B\nstate \"C\" as C\nstate \"D\" as D\nstate \"E\" as E\n" +
				"[*] --> A\n" +
				"A --> B : tau ; a\x00b\n" +
				"A --> C : tau ; a\n" +
				"C --> D : tau ; b\n" +
				"B --> D : tau\n" +
				"D --> E : x\n" +
				"@enduml\n",
			Want: []string{
				"[¬(a\x00b) ∧ ¬(a)] ×",
				"[a\x00b] → E",
				"[a ∧ ¬(b)] ×",
				"[a ∧ b] → E",
			},
		},
		// Postconditions left out would otherwise keep apart every order in
		// which hidden events that change the values interleave.
		"postconditions not asked for do not tell tau paths apart": {
			Diagram: postDiamond,
			Want: []string{
				"→ E",
			},
		},
		"postconditions asked for are collected along the path, and tell tau paths apart": {
			Diagram: postDiamond,
			Options: transtable.Options{Posts: true},
			Want: []string{
				"/ p ⨾ true ⨾ true → E",
				"/ q ⨾ true ⨾ true → E",
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
			if diff := cmp.Diff(testCase.Want, spellCell(got.Rows[0].Cells[0]), cmpopts.EquateEmpty()); diff != "" {
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

// A caller may extend the condition of one outcome without touching that of
// another, even when both were collected along the same path.
func TestBuildGivesEveryOutcomeItsOwnCondition(t *testing.T) {
	// Arrange
	d := csdf.MustParse(`@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
state "E" as E
state "F" as F
[*] --> A
A --> B : tau ; h1
B --> C : tau ; h2
C --> D : tau ; h3
D --> E : a
D --> F : a
@enduml
`)
	table, err := transtable.Build(d, transtable.Options{})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	cell := table.Rows[0].Cells[0]
	toE, toF := cell[len(cell)-2], cell[len(cell)-1]

	// Act
	extendedE := append(toE.Cond, transtable.Literal{Pred: "x"})
	_ = append(toF.Cond, transtable.Literal{Pred: "y"})

	// Assert
	if got := extendedE[len(extendedE)-1].Pred; got != "x" {
		t.Errorf("want the literal appended to the outcome to E to stay x, got %q", got)
	}
}
