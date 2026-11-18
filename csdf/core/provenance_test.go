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

func TestPromoteCarriesTheBlocksOfEachFamily(t *testing.T) {
	// Setup
	blocks := map[string]string{"run": "run_doors", "maint": "maint_doors"}
	p := Promotion{Modes: []string{"run", "maint"}, Init: "run", Families: []Family{{Map: "doors", ID: "loc", Local: door(), Blocks: blocks}}}

	// Execute
	d, _, err := promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(blocks, d.Families[0].Blocks); diff != "" {
		t.Errorf("blocks (-want +got):\n%s", diff)
	}
}

func TestPromoteCountsTheLocalEdgesBesideAMove(t *testing.T) {
	// Setup: DEPART and ARRIVE both go from open to open.
	p := Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{{Map: "locations", ID: "loc", Local: location(t)}}}
	want := map[string]int{"OPEN": 0, "DEPART": 0, "ARRIVE": 1, "CLOSE": 0}

	// Execute
	d, _, err := promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, e := range d.Edges {
		got[e.Event.Channel] = e.Moves[0].Parallel
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parallel (-want +got):\n%s", diff)
	}
}

func TestPromoteCarriesTheLabelsOfTheLocalStates(t *testing.T) {
	// Setup
	local := door()
	local.Labels = map[string]string{"shut": "Shut", "wide": "Wide open"}
	p := Promotion{Modes: []string{"run"}, Init: "run", Families: []Family{{Map: "doors", ID: "loc", Local: local, Blocks: map[string]string{"run": "run_doors"}}}}

	// Execute
	d, _, err := promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(local.Labels, d.Families[0].Labels); diff != "" {
		t.Errorf("labels (-want +got):\n%s", diff)
	}
}
