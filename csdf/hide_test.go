package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHideRenamesTheEventsOfHiddenChannelsToTau(t *testing.T) {
	// Setup: hiding the channel a turns every a-edge internal, whatever its
	// fields, and leaves b visible. A hidden field becomes a value the tau
	// chooses: the clauses still read it by its name.
	d := MustParse(`@startuml
[*] --> s0
s0 --> s1 : a
s1 --> s2 : a?x ; x > 0
s2 --> s0 : b?y
@enduml`)

	// Execute
	got := Hide(d, []string{"a"})

	// Assert
	want := []Event{Tau, Tau, "b?y"}
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

	got := Hide(d, []string{"a"})

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
