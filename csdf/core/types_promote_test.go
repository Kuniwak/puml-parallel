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

func TestPromoteRefusesAChannelOfTwoArities(t *testing.T) {
	// Setup
	p := Promotion{
		Modes: []string{"G"}, Init: "G",
		Channels: map[string][]types.Type{"OPEN": {types.Named("Loc"), types.Named("Loc")}},
		Families: []Family{{Map: "locations", ID: "loc", Key: types.Named("Loc"), Val: types.Named("Location"), Local: typedLocation(t)}},
	}

	// Execute
	_, _, err := Promote(p)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the channel OPEN is declared OPEN : Loc.Loc and OPEN : Loc") {
		t.Errorf("Promote: got %v, want a refusal of the two arities of OPEN", err)
	}
}

func TestPromoteRefusesWhatItCannotType(t *testing.T) {
	typedFamily := Family{Map: "locations", ID: "loc", Key: types.Named("Loc"), Val: types.Named("Location"), Local: typedLocation(t)}
	untypedLocal := typedFamily
	untypedLocal.Local = location(t)
	noKey := typedFamily
	noKey.Key = types.Type{}
	tests := map[string]struct {
		p    Promotion
		want string
	}{
		"a family whose IDs have no type": {Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{noKey}},
			"the family of locations: its IDs or its instances have no type"},
		"a local channel with fields no note declares": {Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{untypedLocal}},
			"DEPART: the field q has no type: the local channel DEPART declares none"},
		"a controller variable with no type": {Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{typedFamily}, Vars: map[string][]string{"G": {"count"}}},
			"the controller variable count has no type"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, _, err := Promote(tt.p)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Promote: got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

func TestPromoteBindsTheInstanceOfAnInternalEventByTheTypeOfItsIDs(t *testing.T) {
	// Setup
	local := &Diagram{States: []string{"none", "a", "b"}, Init: "none", Edges: []Edge{
		{Src: "none", Dst: "a", Event: ev("NEW")}, {Src: "a", Dst: "b", Event: ev(Tau)}}}
	p := Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{{Map: "lamps", ID: "l", Key: types.Named("LampID"), Val: types.Named("Lamp"), Local: local}}}

	// Execute
	d, _, err := Promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range d.Edges {
		if e.Event.Channel == Tau {
			if got := printed(t, e.Guard); !strings.HasPrefix(got, "@exists(l : LampID)") {
				t.Errorf("the guard of the τ is %s, want it to bind l of the type LampID", got)
			}
		}
	}
}

func TestHideRefusesAFieldOfAnUndeclaredChannel(t *testing.T) {
	// Setup
	d := oneState(t, "c", nil, "", loop(t, "c", ev("put", recv("v")), "$v = 0", ""))
	d.Channels = nil

	// Execute
	_, err := Hide(d, []string{"put"})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "put receives v, but declares no type of it") {
		t.Errorf("Hide: got %v, want a refusal of the undeclared field", err)
	}
}

func TestMapTypeRangesTheFieldsOverTheirDeclaredTypes(t *testing.T) {
	// Setup: stock is of the type N, which the environment gives; no type
	// is named like it.
	f := Family{Map: "locations", ID: "loc", Key: types.Named("Loc"), Val: types.Named("Location"), Local: typedLocation(t)}
	env := Env{Types: map[string][]Value{"N": {0, 1}}}

	// Execute
	got, err := MapType(f, []Value{"a"}, env)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 { // absent, or open with a stock of 0 or 1
		t.Errorf("MapType gives %d maps, want 3: %v", len(got), got)
	}
}
