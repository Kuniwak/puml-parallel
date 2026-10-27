package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v3/csdf/lts"
)

func TestReachableKeepsWhatAStepCanReachAndTheMeaning(t *testing.T) {
	// Setup: c is reached by no edge from a; d only from c.
	d := &Diagram{
		States: []string{"a", "b", "c", "d"},
		Vars:   map[string][]string{"c": {"x"}},
		Labels: map[string]string{"a": "A", "c": "C"},
		Init:   "a",
		Edges: []Edge{
			{Src: "a", Dst: "b", Event: ev("go")},
			{Src: "c", Dst: "d", Event: ev("lost")},
			{Src: "b", Dst: "a", Event: ev("back")},
		},
	}
	wantStates := []string{"a", "b"}
	wantEdges := []Edge{d.Edges[0], d.Edges[2]}

	// Execute
	got := Reachable(d)

	// Assert
	if diff := cmp.Diff(wantStates, got.States); diff != "" {
		t.Errorf("States (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantEdges, got.Edges, cmp.AllowUnexported()); diff != "" {
		t.Errorf("Edges (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]string{"a": "A"}, got.Labels); diff != "" {
		t.Errorf("Labels (-want +got):\n%s", diff)
	}
	full, err := Expand(d, bits())
	if err != nil {
		t.Fatal(err)
	}
	pruned, err := Expand(got, bits())
	if err != nil {
		t.Fatal(err)
	}
	if !lts.StableFailuresEqual(full, pruned) {
		t.Error("the meaning changed")
	}
}
