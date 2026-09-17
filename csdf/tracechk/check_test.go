package tracechk_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

func parse(t *testing.T, src string) *csdf.Diagram {
	t.Helper()
	d, err := csdf.Parse(src)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	return d
}

func check(t *testing.T, m tracechk.Match, src string, events ...csdf.Event) tracechk.Result {
	t.Helper()
	return tracechk.Check(m, parse(t, src), tracechk.Trace{Events: events})
}

func TestCheck(t *testing.T) {
	type testCase struct {
		Match   tracechk.Match
		Diagram string
		Trace   []csdf.Event
		// WantSteps is the accepted result; nil when a rejection is wanted.
		WantSteps     []tracechk.Step
		WantRejection *tracechk.Rejection
	}

	linear := `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : x ; g1 ; p1
b --> a : y ; g2 ; p2
@enduml
`
	ax := csdf.Edge{Src: "a", Dst: "b", Event: "x", Guard: "g1", Post: "p1", Line: 5}
	by := csdf.Edge{Src: "b", Dst: "a", Event: "y", Guard: "g2", Post: "p2", Line: 6}

	forked := `@startuml
state "A" as A
state "B" as B
state "C" as C
state "D" as D
[*] --> A
A --> B : 1
A --> C : 1
C --> D : 2
@enduml
`
	a1b := csdf.Edge{Src: "A", Dst: "B", Event: "1", Guard: "true", Post: "true", Line: 7}

	tauThenX := `@startuml
state "a" as a
state "b" as b
state "c" as c
[*] --> a
a --> b : tau ; gt ; pt
b --> c : x
@enduml
`
	abTau := csdf.Edge{Src: "a", Dst: "b", Event: "tau", Guard: "gt", Post: "pt", Line: 6}
	bcX := csdf.Edge{Src: "b", Dst: "c", Event: "x", Guard: "true", Post: "true", Line: 7}

	testCases := map[string]testCase{
		"a linear trace is accepted with one path per prefix": {
			Match: tracechk.MatchExact, Diagram: linear, Trace: []csdf.Event{"x", "y"},
			WantSteps: []tracechk.Step{
				{Index: 0, Event: "x", Paths: []tracechk.PrefixPath{{Dst: "a", Next: []csdf.Edge{ax}}}},
				{Index: 1, Event: "y", Paths: []tracechk.PrefixPath{{Path: tracechk.Path{ax}, Dst: "b", Next: []csdf.Edge{by}}}},
			},
		},
		"the empty trace is accepted with no steps": {
			Match: tracechk.MatchExact, Diagram: linear, Trace: nil,
			WantSteps: []tracechk.Step{},
		},
		"an event no edge carries is rejected there": {
			Match: tracechk.MatchExact, Diagram: linear, Trace: []csdf.Event{"x", "w", "y"},
			WantRejection: &tracechk.Rejection{
				Index: 1, Event: "w",
				States:   []csdf.StateID{"b"},
				Refusing: []csdf.StateID{"b"},
				Enabled:  []csdf.Event{"y"},
				Paths:    []tracechk.PrefixPath{{Path: tracechk.Path{ax}, Dst: "b", Next: nil}},
			},
		},
		"a nondeterministic branch that refuses the next event rejects the trace": {
			// After 1 the diagram may be in B, which is stable and cannot do 2:
			// (<1>, {2}) is a failure, even though C can do 2.
			Match: tracechk.MatchExact, Diagram: forked, Trace: []csdf.Event{"1", "2"},
			WantRejection: &tracechk.Rejection{
				Index: 1, Event: "2",
				States:   []csdf.StateID{"B", "C"},
				Refusing: []csdf.StateID{"B"},
				Enabled:  nil,
				Paths:    []tracechk.PrefixPath{{Path: tracechk.Path{a1b}, Dst: "B", Next: nil}},
			},
		},
		"an unstable state does not refuse: the tau edge is part of the path": {
			Match: tracechk.MatchExact, Diagram: tauThenX, Trace: []csdf.Event{"x"},
			WantSteps: []tracechk.Step{
				{Index: 0, Event: "x", Paths: []tracechk.PrefixPath{
					{Dst: "a", Taus: []csdf.Edge{abTau}, Next: nil},
					{Path: tracechk.Path{abTau}, Dst: "b", Next: []csdf.Edge{bcX}},
				}},
			},
		},
		"a tau cycle yields finitely many paths": {
			Match: tracechk.MatchExact,
			Diagram: `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
a --> a : x
b --> b : x
@enduml
`,
			Trace: []csdf.Event{"x"},
			WantSteps: []tracechk.Step{
				{Index: 0, Event: "x", Paths: []tracechk.PrefixPath{
					{Dst: "a", Taus: []csdf.Edge{{Src: "a", Dst: "b", Event: "tau", Guard: "true", Post: "true", Line: 5}}, Next: []csdf.Edge{{Src: "a", Dst: "a", Event: "x", Guard: "true", Post: "true", Line: 7}}},
					{Path: tracechk.Path{{Src: "a", Dst: "b", Event: "tau", Guard: "true", Post: "true", Line: 5}}, Dst: "b", Taus: []csdf.Edge{{Src: "b", Dst: "a", Event: "tau", Guard: "true", Post: "true", Line: 6}}, Next: []csdf.Edge{{Src: "b", Dst: "b", Event: "x", Guard: "true", Post: "true", Line: 8}}},
				}},
			},
		},
		"prefix match accepts a trace stripped of its parameters": {
			Match: tracechk.MatchPrefix,
			Diagram: `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : insert(coin)
@enduml
`,
			Trace: []csdf.Event{"insert"},
			WantSteps: []tracechk.Step{
				{Index: 0, Event: "insert", Paths: []tracechk.PrefixPath{{Dst: "a", Next: []csdf.Edge{{Src: "a", Dst: "b", Event: "insert(coin)", Guard: "true", Post: "true", Line: 5}}}}},
			},
		},
		"exact match rejects a prefix": {
			Match: tracechk.MatchExact,
			Diagram: `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : insert(coin)
@enduml
`,
			Trace: []csdf.Event{"insert"},
			WantRejection: &tracechk.Rejection{
				Index: 0, Event: "insert",
				States:   []csdf.StateID{"a"},
				Refusing: []csdf.StateID{"a"},
				Enabled:  []csdf.Event{"insert(coin)"},
				Paths:    []tracechk.PrefixPath{{Dst: "a"}},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			result := check(t, testCase.Match, testCase.Diagram, testCase.Trace...)

			// Assert
			if testCase.WantSteps != nil {
				if result.Rejection != nil {
					t.Fatalf("want accepted, got %#v", result.Rejection)
				}
				if diff := cmp.Diff(testCase.WantSteps, result.Steps); diff != "" {
					t.Error(diff)
				}
				return
			}
			if result.Steps != nil {
				t.Errorf("want no steps, got %#v", result.Steps)
			}
			if diff := cmp.Diff(testCase.WantRejection, result.Rejection); diff != "" {
				t.Error(diff)
			}
		})
	}
}
