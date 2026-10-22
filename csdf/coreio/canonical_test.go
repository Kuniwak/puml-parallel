package coreio

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/core"
)

func TestCanonicalOrdersStatesAndEdgesAsTheOldToolsPrinted(t *testing.T) {
	// Setup
	d := &core.Diagram{
		States: []string{"b", "a"},
		Init:   "a",
		Edges: []core.Edge{
			{Src: "b", Dst: "a", Event: core.Event{Channel: "x"}},
			{Src: "a", Dst: "b", Event: core.Event{Channel: "y"}},
			{Src: "a", Dst: "b", Event: core.Event{Channel: "x"}, Guard: mustParse(t, "$v < 2")},
			{Src: "a", Dst: "b", Event: core.Event{Channel: "x"}, Guard: mustParse(t, "$v < 1")},
		},
	}
	wantStates := []string{"a", "b"}
	wantEdges := []core.Edge{d.Edges[3], d.Edges[2], d.Edges[1], d.Edges[0]}

	// Execute
	got, err := Canonical(d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantStates, got.States); diff != "" {
		t.Errorf("States (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantEdges, got.Edges, sameClauses); diff != "" {
		t.Errorf("Edges (-want +got):\n%s", diff)
	}
}
