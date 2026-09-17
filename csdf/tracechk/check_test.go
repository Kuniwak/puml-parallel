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

func TestCheckAcceptsALinearTraceAsOnePath(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : x ; g1 ; p1
b --> a : y ; g2 ; p2
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"x", "y"}})

	// Assert
	if result.Rejection != nil {
		t.Fatalf("want accepted, got %#v", result.Rejection)
	}
	want := []tracechk.Path{{
		{Src: "a", Dst: "b", Event: "x", Guard: "g1", Post: "p1", Line: 5},
		{Src: "b", Dst: "a", Event: "y", Guard: "g2", Post: "p2", Line: 6},
	}}
	if diff := cmp.Diff(want, result.Paths); diff != "" {
		t.Error(diff)
	}
}

func TestCheckRejectsWhereNoEdgeCarriesTheEvent(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : x
b --> a : y
b --> b : z
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"x", "w", "y"}})

	// Assert
	if result.Paths != nil {
		t.Errorf("want no result.Paths, got %#v", result.Paths)
	}
	want := &tracechk.Rejection{
		Index:   1,
		Event:   "w",
		States:  []csdf.StateID{"b"},
		Enabled: []csdf.Event{"y", "z"},
	}
	if diff := cmp.Diff(want, result.Rejection); diff != "" {
		t.Error(diff)
	}
}

func TestCheckFollowsTauEdgesAndReportsThemInThePath(t *testing.T) {
	// Arrange: x is only enabled after a silent step, and the tau edge is part
	// of the path because its predicates constrain the trace too.
	d := parse(t, `@startuml
state "a" as a
state "b" as b
state "c" as c
[*] --> a
a --> b : tau ; gt ; pt
b --> c : x
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"x"}})

	// Assert
	if result.Rejection != nil {
		t.Fatalf("want accepted, got %#v", result.Rejection)
	}
	want := []tracechk.Path{{
		{Src: "a", Dst: "b", Event: "tau", Guard: "gt", Post: "pt", Line: 6},
		{Src: "b", Dst: "c", Event: "x", Guard: "true", Post: "true", Line: 7},
	}}
	if diff := cmp.Diff(want, result.Paths); diff != "" {
		t.Error(diff)
	}
}

func TestCheckReportsTheTauClosedStatesOnRejection(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> b : y
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"x"}})

	// Assert
	want := &tracechk.Rejection{
		Index:   0,
		Event:   "x",
		States:  []csdf.StateID{"a", "b"},
		Enabled: []csdf.Event{"y"},
	}
	if diff := cmp.Diff(want, result.Rejection); diff != "" {
		t.Error(diff)
	}
}

func TestCheckReturnsEveryPathOfANondeterministicTrace(t *testing.T) {
	// Arrange: two edges carry x out of a, so the trace has two result.Paths and the
	// obligation is a disjunction over them.
	d := parse(t, `@startuml
state "a" as a
state "b" as b
state "c" as c
[*] --> a
a --> b : x ; g1
a --> c : x ; g2
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"x"}})

	// Assert
	if result.Rejection != nil {
		t.Fatalf("want accepted, got %#v", result.Rejection)
	}
	want := []tracechk.Path{
		{{Src: "a", Dst: "b", Event: "x", Guard: "g1", Post: "true", Line: 6}},
		{{Src: "a", Dst: "c", Event: "x", Guard: "g2", Post: "true", Line: 7}},
	}
	if diff := cmp.Diff(want, result.Paths); diff != "" {
		t.Error(diff)
	}
}

func TestCheckDoesNotLoopOnATauCycle(t *testing.T) {
	// Arrange: a tau cycle between a and b. A path never revisits a state
	// within one run of tau edges, so the round trip a -tau-> b -tau-> a is not
	// a path and the only path is the two direct x edges.
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
a --> a : x
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"x", "x"}})

	// Assert
	if result.Rejection != nil {
		t.Fatalf("want accepted, got %#v", result.Rejection)
	}
	if len(result.Paths) != 1 {
		t.Errorf("want 1 path, got %d: %#v", len(result.Paths), result.Paths)
	}
}

func TestCheckAcceptsTheEmptyTrace(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
[*] --> a
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{})

	// Assert
	if result.Rejection != nil {
		t.Fatalf("want accepted, got %#v", result.Rejection)
	}
	if len(result.Paths) != 1 || len(result.Paths[0]) != 0 {
		t.Errorf("want one empty path, got %#v", result.Paths)
	}
}

func TestCheckPrefixMatchAcceptsATraceEventThatPrefixesTheDiagramEvent(t *testing.T) {
	// Arrange: the trace was stripped of its parameters (e.g. with qhs), so
	// "insert" has to match "insert(coin)"; "reset" still matches "reset".
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : insert(coin)
b --> a : reset
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchPrefix, d, tracechk.Trace{Events: []csdf.Event{"insert", "reset"}})

	// Assert
	if result.Rejection != nil {
		t.Fatalf("want accepted, got %#v", result.Rejection)
	}
	if len(result.Paths) != 1 || len(result.Paths[0]) != 2 {
		t.Errorf("want one path of two edges, got %#v", result.Paths)
	}
}

func TestCheckExactMatchRejectsAPrefix(t *testing.T) {
	// Arrange
	d := parse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : insert(coin)
@enduml
`)

	// Act
	result := tracechk.Check(tracechk.MatchExact, d, tracechk.Trace{Events: []csdf.Event{"insert"}})

	// Assert
	if result.Rejection == nil || result.Rejection.Index != 0 {
		t.Errorf("want result.Rejection at 0, got %#v", result.Rejection)
	}
}
