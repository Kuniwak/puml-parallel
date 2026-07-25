package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// ignoreEdgeLine drops the source-line field when comparing witness edges, which
// is positional metadata rather than part of the livelock witness identity.
var ignoreEdgeLine = cmpopts.IgnoreFields(Edge{}, "Line")

func TestCheckLivelockFreeReportsFreeWhenNoTauEdges(t *testing.T) {
	// Setup: a visible-only chain has no tau edges, so it is livelock free.
	d := MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if !ok {
		t.Errorf("want livelock free, got witness %+v", witness)
	}
	if witness != nil {
		t.Errorf("want nil witness, got %+v", witness)
	}
}

func TestCheckLivelockFreeDetectsTauSelfLoop(t *testing.T) {
	// Setup: a tau self-loop on the start state is the degenerate livelock.
	d := MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : tau
@enduml
`)
	want := &Livelock{
		Cycle: []Edge{{Src: "s0", Dst: "s0", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue}},
	}

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
	if diff := cmp.Diff(want, witness, ignoreEdgeLine); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeDetectsTauTwoCycle(t *testing.T) {
	// Setup: a two-state tau cycle a -> b -> a.
	d := MustParse(`@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)
	want := &Livelock{
		Cycle: []Edge{
			{Src: "a", Dst: "b", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue},
			{Src: "b", Dst: "a", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue},
		},
	}

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
	if diff := cmp.Diff(want, witness, ignoreEdgeLine); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeIgnoresMixedCycleWithVisibleEvent(t *testing.T) {
	// Setup: a cycle s0 -> s1 -> s0 containing a visible event is not a livelock.
	d := MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : tau
s1 --> s0 : e
@enduml
`)

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if !ok {
		t.Errorf("want livelock free, got witness %+v", witness)
	}
}

func TestCheckLivelockFreeIgnoresUnreachableTauCycle(t *testing.T) {
	// Setup: a tau cycle x <-> y exists but is not reachable from the start state.
	d := MustParse(`@startuml
state "s0" as s0
state "s1" as s1
state "x" as x
state "y" as y
[*] --> s0
s0 --> s1 : a
x --> y : tau
y --> x : tau
@enduml
`)

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if !ok {
		t.Errorf("want livelock free, got witness %+v", witness)
	}
}

func TestCheckLivelockFreeBuildsStemThroughVisibleEvents(t *testing.T) {
	// Setup: a visible event leads from the start state into a tau cycle.
	d := MustParse(`@startuml
state "s0" as s0
state "sa" as sa
state "sb" as sb
[*] --> s0
s0 --> sa : a
sa --> sb : tau
sb --> sa : tau
@enduml
`)
	want := &Livelock{
		Stem: []Edge{{Src: "s0", Dst: "sa", Event: "a", Guard: PredicateTrue, Post: PredicateTrue}},
		Cycle: []Edge{
			{Src: "sa", Dst: "sb", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue},
			{Src: "sb", Dst: "sa", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue},
		},
	}

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
	if diff := cmp.Diff(want, witness, ignoreEdgeLine); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeChoosesDeterministicCycle(t *testing.T) {
	// Setup: two disjoint reachable tau cycles; the witness must be the
	// deterministically-first one (smallest state IDs) and stable across runs.
	d := MustParse(`@startuml
state "s0" as s0
state "a0" as a0
state "a1" as a1
state "b0" as b0
state "b1" as b1
[*] --> s0
s0 --> a0 : a
a0 --> a1 : tau
a1 --> a0 : tau
s0 --> b0 : b
b0 --> b1 : tau
b1 --> b0 : tau
@enduml
`)
	want := &Livelock{
		Stem: []Edge{{Src: "s0", Dst: "a0", Event: "a", Guard: PredicateTrue, Post: PredicateTrue}},
		Cycle: []Edge{
			{Src: "a0", Dst: "a1", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue},
			{Src: "a1", Dst: "a0", Event: Tau, Guard: PredicateTrue, Post: PredicateTrue},
		},
	}

	// Execute & Assert: stable across repeated runs (Go map iteration is randomized).
	for i := 0; i < 5; i++ {
		witness, ok := CheckLivelockFree(d)
		if ok {
			t.Fatal("want livelock detected, got livelock free")
		}
		if diff := cmp.Diff(want, witness, ignoreEdgeLine); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestCheckLivelockFreeIgnoresEndEdge(t *testing.T) {
	// Setup: an end edge must not be rejected; the tau cycle is still detected.
	d := MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : tau
s0 --> [*]
@enduml
`)

	// Execute
	_, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
}

func TestCheckLivelockFreeReportsFreeForEndEdgeWithoutTauCycle(t *testing.T) {
	// Setup: a terminating diagram with no tau edges is livelock free.
	d := MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
s1 --> [*]
@enduml
`)

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if !ok {
		t.Errorf("want livelock free, got witness %+v", witness)
	}
}

func TestCheckLivelockFreeHandlesSingleStateDiagram(t *testing.T) {
	// Setup: a single state with no edges is trivially livelock free.
	d := MustParse(`@startuml
state "s0" as s0
[*] --> s0
@enduml
`)

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if !ok {
		t.Errorf("want livelock free, got witness %+v", witness)
	}
}
