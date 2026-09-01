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
	// Setup: the caller keeps the unhidden diagram usable after hiding.
	d := MustParse(`@startuml
[*] --> s0
s0 --> s1 : a
@enduml`)

	Hide(d, []Event{"a"})

	if got := d.Edges[0].Event; got != Event("a") {
		t.Errorf("want a, got %s", got)
	}
}
