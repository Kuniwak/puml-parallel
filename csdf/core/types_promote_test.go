package core

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/types"
)

// typedLocation is location with its variable and channels declared.
func typedLocation(t *testing.T) *Diagram {
	return typed(location(t), map[string]types.Type{"stock": types.Named("N")}, map[string][]types.Type{"DEPART": {types.Named("Q")}, "ARRIVE": {types.Named("Q")}})
}

func TestPromoteDeclaresTheTypesOfItsMapsAndChannels(t *testing.T) {
	// Setup
	p := Promotion{
		Modes: []string{"G"}, Init: "G",
		Vars:     map[string][]string{"G": {"count"}},
		Types:    map[string]types.Type{"count": types.Named("N")},
		Channels: map[string][]types.Type{"HALT": {types.Named("Why")}},
		Families: []Family{{Map: "locations", ID: "loc", Key: types.Named("Loc"), Val: types.Named("Location"), Local: typedLocation(t)}},
		Syncs: []Sync{{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "to", Event: "ARRIVE"}}}},
		Edges: []Edge{{Src: "G", Dst: "G", Event: ev("HALT", recv("why"))}},
	}

	// Execute
	got, _, err := Promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := map[string]types.Type{"locations": types.MapOf(types.Named("Loc"), types.Named("Location")), "count": types.Named("N")}
	if diff := cmp.Diff(wantTypes, got.Types); diff != "" {
		t.Errorf("Types (-want +got):\n%s", diff)
	}
	loc, q := types.Named("Loc"), types.Named("Q")
	wantChannels := map[string][]types.Type{"OPEN": {loc}, "CLOSE": {loc}, "MOVE": {loc, loc, q}, "HALT": {types.Named("Why")}}
	if diff := cmp.Diff(wantChannels, got.Channels); diff != "" {
		t.Errorf("Channels (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(types.Named("Loc"), got.Families[0].KeyType); diff != "" {
		t.Errorf("KeyType (-want +got):\n%s", diff)
	}
}

func TestPromoteRefusesAFieldOfTwoTypes(t *testing.T) {
	// Setup
	local := typedLocation(t)
	local.Channels["ARRIVE"] = []types.Type{types.Named("R")}
	p := Promotion{
		Modes: []string{"G"}, Init: "G",
		Families: []Family{{Map: "locations", ID: "loc", Key: types.Named("Loc"), Val: types.Named("Location"), Local: local}},
		Syncs: []Sync{{Event: "MOVE", Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}, {Map: "locations", ID: "to", Event: "ARRIVE"}}}},
	}

	// Execute
	_, _, err := Promote(p)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the field q of MOVE is of the type Q and of the type R") {
		t.Errorf("Promote: got %v, want a refusal of the field q", err)
	}
}

func TestRestrictKeepsTheTypesOfWhatItKeeps(t *testing.T) {
	// Setup
	d := &Diagram{States: []string{"G"}, Init: "G", Vars: map[string][]string{"G": {"a", "b"}},
		Types:    map[string]types.Type{"a": types.MapOf(types.Named("K"), types.Named("A")), "b": types.MapOf(types.Named("K"), types.Named("B"))},
		Channels: map[string][]types.Type{"PUT": {types.Named("K")}},
		Edges:    []Edge{{Src: "G", Dst: "G", Event: ev("PUT", recv("k")), Post: mustParse(t, "@frame(a', a, k) ∧ @eq(b', b)")}}}

	// Execute
	got, _, err := Restrict(d, []string{"a", "b"}, []string{"a"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]types.Type{"a": d.Types["a"]}, got.Types); diff != "" {
		t.Errorf("Types (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(d.Channels, got.Channels); diff != "" {
		t.Errorf("Channels (-want +got):\n%s", diff)
	}
}
