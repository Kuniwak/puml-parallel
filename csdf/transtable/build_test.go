package transtable_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// The variables of a condition, and shorthands to build conditions with.
var (
	x  = transtable.ValuesAfterStep(0)
	x1 = transtable.ValuesAfterStep(1)
	x2 = transtable.ValuesAfterStep(2)
	x3 = transtable.ValuesAfterStep(3)
	c  = transtable.OfferedParams
	c1 = transtable.ParamsOfStep(1)
	c2 = transtable.ParamsOfStep(2)
	c3 = transtable.ParamsOfStep(3)
	c4 = transtable.ParamsOfStep(4)
	xn = transtable.NextValues

	q   = logic.Quoted
	and = logic.And
	not = logic.Not
)

func ex(vars []logic.Var, body logic.Formula) logic.Formula { return logic.Exists(vars, body) }
func vs(vars ...logic.Var) []logic.Var                      { return vars }

func goTo(cond logic.Formula, s csdf.StateID) transtable.Outcome {
	return transtable.Outcome{Cond: cond, Result: transtable.Goto{State: s}}
}

func refuse(cond logic.Formula) transtable.Outcome {
	return transtable.Outcome{Cond: cond, Result: transtable.Refuse{}}
}

// equalFormulas compares conditions as formulas, by logic.Equal, and leaves
// the spelling of them out of what Build is tested for.
var equalFormulas = cmp.Comparer(logic.Equal)

// lines spells outcomes, only to say what went wrong.
func linesOf(os []transtable.Outcome) string {
	f := format(transtable.NotationLogical)
	ls := make([]string, len(os))
	for i, o := range os {
		ls[i] = f.Line(o)
	}
	return strings.Join(ls, "\n")
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
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{goTo(logic.True, "s1")}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
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
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{
						goTo(q("g", c, x), "s1"),
						refuse(not(q("g", c, x))),
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
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
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{
						goTo(q("g1", c, x), "s1"),
						goTo(q("g2", c, x), "s2"),
						refuse(and(not(q("g1", c, x)), not(q("g2", c, x)))),
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
					{State: "s2", Name: "S2", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
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
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{
						goTo(q("g", c, x), "s1"),
						goTo(logic.True, "s2"),
					}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
					{State: "s2", Name: "S2", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
				},
			},
		},
		"a postcondition is applied to the parameters and the values before and after": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
[*] --> s0
s0 --> s1 : a ; true ; p
@enduml
`,
			Want: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{goTo(q("p", c, x, xn), "s1")}}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
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
		// An end edge has no event, so its guard reads the values only.
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
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}, transtable.TerminationColumn{}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{
						{goTo(logic.True, "s1")},
						{refuse(logic.True)},
					}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{
						{refuse(logic.True)},
						{
							{Cond: q("g", x), Result: transtable.Terminate{}},
							refuse(not(q("g", x))),
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
				Columns: []transtable.Column{transtable.EventColumn{Event: "BOOK"}, transtable.EventColumn{Event: "REPORT"}},
				Rows: []transtable.Row{
					{State: "idle", Name: "Idle", Cells: [][]transtable.Outcome{
						{goTo(logic.True, "counting")},
						{refuse(logic.True)},
					}},
					// Counting never refuses by itself, since its tau always
					// may fire; Fixed, which the tau leads to, refuses BOOK.
					{State: "counting", Name: "Counting", Cells: [][]transtable.Outcome{
						{goTo(logic.True, "counting"), refuse(logic.True)},
						{goTo(logic.True, "idle")},
					}},
					{State: "fixed", Name: "Fixed", Cells: [][]transtable.Outcome{
						{refuse(logic.True)},
						{goTo(logic.True, "idle")},
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
			got, err := transtable.Build(d)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if !cmp.Equal(testCase.Want, got, equalFormulas, cmpopts.EquateEmpty()) {
				t.Errorf("want %s, got %s", tableText(testCase.Want), tableText(got))
			}
		})
	}
}

// tableText spells a table, only to say what went wrong.
func tableText(t *transtable.Table) string {
	var sb strings.Builder
	for _, c := range t.Columns {
		sb.WriteString("\n  column " + c.Header())
	}
	for _, row := range t.Rows {
		sb.WriteString("\n  row " + string(row.State) + " " + row.Name)
		for _, cell := range row.Cells {
			sb.WriteString("\n    " + strings.ReplaceAll(linesOf(cell), "\n", "\n    "))
		}
	}
	for _, s := range t.Unreachable {
		sb.WriteString("\n  unreachable " + string(s))
	}
	return sb.String()
}

// TestBuildFirstCell looks at one cell only, that of the start state under the
// first column, where the whole table would bury the point.
func TestBuildFirstCell(t *testing.T) {
	type testCase struct {
		Diagram string
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
				refuse(not(ex(vs(c1), q("h1", c1, x)))),
				refuse(ex(vs(c1, x1), and(q("h1", c1, x), not(ex(vs(c2), q("h2", c2, x1)))))),
				goTo(ex(vs(c1, x1, c2, x2), and(q("h1", c1, x), q("h2", c2, x1), q("g", c, x2))), "D"),
				refuse(ex(vs(c1, x1, c2, x2), and(q("h1", c1, x), q("h2", c2, x1), not(q("g", c, x2))))),
			},
		},
		"a postcondition along a tau path is applied to the values before and after its step": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : tau ; h ; x' = x + 1
B --> C : a ; g ; y' = x
@enduml
`,
			Want: []transtable.Outcome{
				refuse(not(ex(vs(c1), q("h", c1, x)))),
				goTo(ex(vs(c1, x1), and(q("h", c1, x), q("x' = x + 1", c1, x, x1), q("g", c, x1), q("y' = x", c, x1, xn))), "C"),
				refuse(ex(vs(c1, x1), and(q("h", c1, x), q("x' = x + 1", c1, x, x1), not(q("g", c, x1))))),
			},
		},
		// A true postcondition leaves the values after it free, so a later
		// predicate reads values that are only bound.
		"true guards and postconditions add nothing, and the values they leave free stay bound": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : tau
B --> C : a ; true ; y' = 0
C --> A : b
@enduml
`,
			Want: []transtable.Outcome{
				goTo(ex(vs(x1), q("y' = 0", c, x1, xn)), "C"),
			},
		},
		// Hiding interleaved events makes tau diamonds, and a walk that took
		// every way round each of them would grow exponentially. Two ways that
		// reach a state along the same guards and postconditions lead to the
		// same outcomes, so the second is not walked.
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
			Want: []transtable.Outcome{goTo(logic.True, "E")},
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
			Want: []transtable.Outcome{
				refuse(and(not(ex(vs(c1), q("g1", c1, x))), not(ex(vs(c1), q("g2", c1, x))))),
				goTo(ex(vs(c1), q("g1", c1, x)), "E"),
				goTo(ex(vs(c1), q("g2", c1, x)), "E"),
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
			Want: []transtable.Outcome{
				goTo(q("g", c, x), "B"),
				refuse(and(not(ex(vs(c1), q("h", c1, x))), not(q("g", c, x)))),
				refuse(ex(vs(c1), q("h", c1, x))),
			},
		},
		// The path to D is long enough for its steps to have spare capacity,
		// so the two tau edges out of D would write their steps over each
		// other if a branch extended the path in place.
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
			Want: []transtable.Outcome{
				refuse(not(ex(vs(c1), q("h1", c1, x)))),
				refuse(ex(vs(c1, x1), and(q("h1", c1, x), not(ex(vs(c2), q("h2", c2, x1)))))),
				refuse(ex(vs(c1, x1, c2, x2), and(q("h1", c1, x), q("h2", c2, x1), not(ex(vs(c3), q("h3", c3, x2)))))),
				refuse(ex(vs(c1, x1, c2, x2, c3, x3), and(q("h1", c1, x), q("h2", c2, x1), q("h3", c3, x2),
					not(ex(vs(c4), q("k1", c4, x3))), not(ex(vs(c4), q("k2", c4, x3)))))),
				goTo(ex(vs(c1, x1, c2, x2, c3, x3, c4), and(q("h1", c1, x), q("h2", c2, x1), q("h3", c3, x2), q("k1", c4, x3))), "F"),
				goTo(ex(vs(c1, x1, c2, x2, c3, x3, c4), and(q("h1", c1, x), q("h2", c2, x1), q("h3", c3, x2), q("k2", c4, x3))), "F"),
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
			Want: []transtable.Outcome{
				refuse(and(not(ex(vs(c1), q("a\x00b", c1, x))), not(ex(vs(c1), q("a", c1, x))))),
				goTo(ex(vs(c1), q("a\x00b", c1, x)), "E"),
				refuse(ex(vs(c1, x1), and(q("a", c1, x), not(ex(vs(c2), q("b", c2, x1)))))),
				goTo(ex(vs(c1, x1, c2), and(q("a", c1, x), q("b", c2, x1))), "E"),
			},
		},
		// A postcondition is part of the condition, so two ways that differ
		// in one alone are two outcomes.
		"postconditions along a tau path tell the paths apart": {
			Diagram: postDiamond,
			Want: []transtable.Outcome{
				goTo(ex(vs(c1, x1), q("p", c1, x, x1)), "E"),
				goTo(ex(vs(c1, x1), q("q", c1, x, x1)), "E"),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := csdf.MustParse(testCase.Diagram)

			// Act
			got, err := transtable.Build(d)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			cell := got.Rows[0].Cells[0]
			if !cmp.Equal(testCase.Want, cell, equalFormulas) {
				t.Errorf("want\n%s\ngot\n%s", linesOf(testCase.Want), linesOf(cell))
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
	_, err := transtable.Build(d)

	// Assert
	var livelock *transtable.LivelockError
	if !errors.As(err, &livelock) {
		t.Fatalf("want a *LivelockError, got %v", err)
	}
	if want := "A -> B -> A"; !strings.Contains(err.Error(), want) {
		t.Errorf("want the cycle %q in the message, got %q", want, err.Error())
	}
}

// A row is a state with its name and state variables, which an undeclared
// state has none of, so a diagram with one is not tabulated.
func TestBuildRefusesAnUndeclaredState(t *testing.T) {
	type testCase struct {
		Diagram string
		Want    []csdf.StateID
	}

	testCases := map[string]testCase{
		"one an edge leads to": {
			Diagram: `@startuml
state "A" as A
[*] --> A
A --> B : a
@enduml
`,
			Want: []csdf.StateID{"B"},
		},
		"unreachable ones an edge joins": {
			Diagram: `@startuml
state "A" as A
[*] --> A
Z --> Y : b
@enduml
`,
			Want: []csdf.StateID{"Y", "Z"},
		},
		"one an end edge leaves": {
			Diagram: `@startuml
state "A" as A
[*] --> A
E --> [*]
@enduml
`,
			Want: []csdf.StateID{"E"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := csdf.MustParse(testCase.Diagram)

			// Act
			_, err := transtable.Build(d)

			// Assert
			var undeclared *transtable.UndeclaredStateError
			if !errors.As(err, &undeclared) {
				t.Fatalf("want an *UndeclaredStateError, got %v", err)
			}
			if diff := cmp.Diff(testCase.Want, undeclared.States); diff != "" {
				t.Error(diff)
			}
		})
	}
}
