package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCheckLivelockFreeReportsFreeWhenNoTauEdges(t *testing.T) {
	// Setup: a visible-only chain has no tau edges, so it is livelock free.
	d := mustParse(t, `@startuml
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
	d := mustParse(t, `@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : tau
@enduml
`)
	want := &Livelock{
		Cycle: []Edge{{Src: "s0", Dst: "s0", Event: Tau, Guard: True, Post: True}},
	}

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
	if diff := cmp.Diff(want, witness); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeDetectsTauTwoCycle(t *testing.T) {
	// Setup: a two-state tau cycle a -> b -> a.
	d := mustParse(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)
	want := &Livelock{
		Cycle: []Edge{
			{Src: "a", Dst: "b", Event: Tau, Guard: True, Post: True},
			{Src: "b", Dst: "a", Event: Tau, Guard: True, Post: True},
		},
	}

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
	if diff := cmp.Diff(want, witness); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeIgnoresMixedCycleWithVisibleEvent(t *testing.T) {
	// Setup: a cycle s0 -> s1 -> s0 containing a visible event is not a livelock.
	d := mustParse(t, `@startuml
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
	d := mustParse(t, `@startuml
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
	d := mustParse(t, `@startuml
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
		Stem: []Edge{{Src: "s0", Dst: "sa", Event: "a", Guard: True, Post: True}},
		Cycle: []Edge{
			{Src: "sa", Dst: "sb", Event: Tau, Guard: True, Post: True},
			{Src: "sb", Dst: "sa", Event: Tau, Guard: True, Post: True},
		},
	}

	// Execute
	witness, ok := CheckLivelockFree(d)

	// Assert
	if ok {
		t.Error("want livelock detected, got livelock free")
	}
	if diff := cmp.Diff(want, witness); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeChoosesDeterministicCycle(t *testing.T) {
	// Setup: two disjoint reachable tau cycles; the witness must be the
	// deterministically-first one (smallest state IDs) and stable across runs.
	d := mustParse(t, `@startuml
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
		Stem: []Edge{{Src: "s0", Dst: "a0", Event: "a", Guard: True, Post: True}},
		Cycle: []Edge{
			{Src: "a0", Dst: "a1", Event: Tau, Guard: True, Post: True},
			{Src: "a1", Dst: "a0", Event: Tau, Guard: True, Post: True},
		},
	}

	// Execute & Assert: stable across repeated runs (Go map iteration is randomized).
	for i := 0; i < 5; i++ {
		witness, ok := CheckLivelockFree(d)
		if ok {
			t.Fatal("want livelock detected, got livelock free")
		}
		if diff := cmp.Diff(want, witness); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestCheckLivelockFreeIgnoresEndEdge(t *testing.T) {
	// Setup: an end edge must not be rejected; the tau cycle is still detected.
	d := mustParse(t, `@startuml
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
	d := mustParse(t, `@startuml
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

func TestRenderLivelockFormatsStemAndCycle(t *testing.T) {
	// Setup: a witness with a visible stem leading into a tau cycle.
	w := &Livelock{
		Stem: []Edge{{Src: "s0", Dst: "sa", Event: "a"}},
		Cycle: []Edge{
			{Src: "sa", Dst: "sb", Event: Tau},
			{Src: "sb", Dst: "sa", Event: Tau},
		},
	}
	want := "s0 --a--> sa\ncycle:\nsa --tau--> sb\nsb --tau--> sa\n"

	// Execute & Assert
	if diff := cmp.Diff(want, RenderLivelock(w)); diff != "" {
		t.Error(diff)
	}
}

func TestRenderLivelockFormatsSelfLoopWithEmptyStem(t *testing.T) {
	// Setup: a self-loop witness has no stem, so only the cycle block is rendered.
	w := &Livelock{
		Cycle: []Edge{{Src: "s0", Dst: "s0", Event: Tau}},
	}
	want := "cycle:\ns0 --tau--> s0\n"

	// Execute & Assert
	if diff := cmp.Diff(want, RenderLivelock(w)); diff != "" {
		t.Error(diff)
	}
}

func TestCheckLivelockFreeHandlesSingleStateDiagram(t *testing.T) {
	// Setup: a single state with no edges is trivially livelock free.
	d := mustParse(t, `@startuml
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
