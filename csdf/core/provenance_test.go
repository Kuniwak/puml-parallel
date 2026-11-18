package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPromoteRecordsTheMovesOfEachEdge(t *testing.T) {
	// Setup: OPENS and LOST alone; INSTALL syncs a door's NEW with a key's
	// CUT; HALT is the controller's own.
	p := Promotion{Modes: []string{"run", "maint"}, Init: "run",
		Families: []Family{{Map: "doors", ID: "loc", Local: door()}, {Map: "keys", ID: "s", Local: key()}},
		Syncs:    []Sync{{Event: "INSTALL", Targets: []Target{{Map: "doors", ID: "loc", Event: "NEW"}, {Map: "keys", ID: "s", Event: "CUT"}}}},
		Edges:    []Edge{{Src: "run", Dst: "maint", Event: ev("HALT")}}}
	want := map[string][]Move{
		"run LOST":    {{Map: "keys", Edge: 1, Src: "held", Dst: "none", Kind: Deletion}},
		"run OPENS":   {{Map: "doors", Edge: 1, Src: "shut", Dst: "wide", Kind: Stay}},
		"run INSTALL": {{Map: "doors", Edge: 0, Src: "none", Dst: "shut", Kind: Creation}, {Map: "keys", Edge: 0, Src: "none", Dst: "held", Kind: Creation}},
		"run HALT":    nil,
	}

	// Execute
	d, _, err := promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]Move{}
	for _, e := range d.Edges {
		k := e.Src + " " + e.Event.Channel
		if _, ok := want[k]; ok {
			got[k] = e.Moves
		}
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("moves (-want +got):\n%s", diff)
	}
}
