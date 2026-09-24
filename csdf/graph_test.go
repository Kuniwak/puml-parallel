package csdf_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Declared out of canonical order, so that the graph has to put them in it.
const unsortedGraph = `@startuml
state "a" as a
state "b" as b
state "c" as c
state "z" as z
[*] --> a
a --> c : y
a --> b : tau
a --> b : x
b --> c : tau
z --> a : x
@enduml
`

func TestGraphOutListsTheEdgesOfAStateInCanonicalOrder(t *testing.T) {
	// Arrange
	g := csdf.NewGraph(csdf.MustParse(unsortedGraph))

	// Act
	got := events(g.Out("a"))

	// Assert
	if diff := cmp.Diff([]csdf.Event{"tau", "x", "y"}, got); diff != "" {
		t.Error(diff)
	}
}

func TestGraphTausListsTheTauEdgesOfAState(t *testing.T) {
	// Arrange
	g := csdf.NewGraph(csdf.MustParse(unsortedGraph))

	// Act
	got := g.Taus("a")

	// Assert
	if len(got) != 1 || got[0].Dst != "b" || got[0].Event != csdf.Tau {
		t.Errorf("want the one tau edge a --> b, got %v", got)
	}
}

func TestGraphReachableWalksBreadthFirstInCanonicalOrder(t *testing.T) {
	// Arrange
	g := csdf.NewGraph(csdf.MustParse(unsortedGraph))

	// Act
	got := g.Reachable("a")

	// Assert
	if diff := cmp.Diff([]csdf.StateID{"a", "b", "c"}, got); diff != "" {
		t.Error(diff)
	}
}

func TestGraphTauClosureFollowsTauEdgesOnly(t *testing.T) {
	// Arrange
	g := csdf.NewGraph(csdf.MustParse(unsortedGraph))

	// Act
	got := g.TauClosure(map[csdf.StateID]struct{}{"a": {}})

	// Assert
	want := map[csdf.StateID]struct{}{"a": {}, "b": {}, "c": {}}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Error(diff)
	}
}

func TestLivelockCycleStatesNamesTheCycleClosedOnItself(t *testing.T) {
	// Arrange
	d := csdf.MustParse(`@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)
	livelock, ok := csdf.CheckLivelockFree(d)
	if ok {
		t.Fatal("want a livelock, got none")
	}

	// Act
	got := livelock.CycleStates()

	// Assert
	if diff := cmp.Diff([]csdf.StateID{"a", "b", "a"}, got); diff != "" {
		t.Error(diff)
	}
}

func events(edges []csdf.Edge) []csdf.Event {
	es := make([]csdf.Event, len(edges))
	for i, e := range edges {
		es[i] = e.Event
	}
	return es
}
