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
