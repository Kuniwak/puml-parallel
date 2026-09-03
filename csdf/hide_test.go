package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHideRenamesHiddenEventsToTau(t *testing.T) {
	// Setup: hiding "a" turns the a-edge internal and leaves "b" visible.
	d := MustParse(`@startuml
[*] --> s0
s0 --> s1 : a
s1 --> s2 : b
@enduml`)

	got := Hide(d, []Event{"a"})

	want := []Event{Tau, "b"}
	events := make([]Event, 0, len(got.Edges))
	for _, e := range got.Edges {
		events = append(events, e.Event)
	}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Error(diff)
	}
}

func TestHideDoesNotModifyTheInputDiagram(t *testing.T) {
	// Setup: the caller keeps the unhidden diagram usable after hiding, so
	// nothing reachable from the result may alias the input.
	d := MustParse(`@startuml
state "s0" as s0
s0 : n ; Int
state "s1" as s1
[*] --> s0
s0 --> s1 : a
s1 --> [*]
@enduml`)

	got := Hide(d, []Event{"a"})

	got.Edges[0].Event = "mutated"
	got.States["s0"].Vars[0].Name = "mutated"
	got.EndEdge.Guard = "mutated"

	if e := d.Edges[0].Event; e != Event("a") {
		t.Errorf("want a, got %s", e)
	}
	if v := d.States["s0"].Vars[0].Name; v != Var("n") {
		t.Errorf("want n, got %s", v)
	}
	if g := d.EndEdge.Guard; !IsTrue(g) {
		t.Errorf("want true, got %s", g)
	}
}
