package core

import (
	"strings"
	"testing"
)

func TestComposeParallelRejectsEndEdges(t *testing.T) {
	// Setup
	left := Diagram{
		States: map[StateID]State{
			"left": {ID: "left", Name: "Left"},
		},
		StartEdge: StartEdge{Dst: "left"},
		EndEdge:   &EndEdge{Src: "left"},
	}
	right := Diagram{
		States: map[StateID]State{
			"right": {ID: "right", Name: "Right"},
		},
		StartEdge: StartEdge{Dst: "right"},
	}

	// Execute
	_, err := ComposeParallel([]Diagram{left, right}, nil)

	// Assert
	if err == nil {
		t.Fatal("ComposeParallel() error = nil, want end-edge rejection")
	}
	if !strings.Contains(err.Error(), "end edges are not supported") {
		t.Errorf("ComposeParallel() error = %q, want end-edge rejection", err)
	}

	// Teardown: no resources to release.
}

func TestStatePairPreservesStateVarTypes(t *testing.T) {
	// Setup
	pair := StatePair{
		Left: State{
			ID:   "left",
			Name: "Left",
			Vars: []StateVar{{Name: "ready", Type: "bool"}},
		},
		Right: State{
			ID:   "right",
			Name: "Right",
			Vars: []StateVar{{Name: "count", Type: "int"}},
		},
	}

	// Execute
	state := pair.State()

	// Assert
	want := []StateVar{
		{Name: "ready", Type: "bool"},
		{Name: "count", Type: "int"},
	}
	if len(state.Vars) != len(want) {
		t.Fatalf("StatePair.State() vars = %#v, want %#v", state.Vars, want)
	}
	for i := range want {
		if state.Vars[i] != want[i] {
			t.Errorf("StatePair.State() vars[%d] = %#v, want %#v", i, state.Vars[i], want[i])
		}
	}

	// Teardown: no resources to release.
}
