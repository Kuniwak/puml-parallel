package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// observedLocation is location observing the view live of its one live
// state and nothing else.
func observedLocation(t *testing.T, vars ...string) *Diagram {
	d := location(t)
	d.Observes = &Observation{Views: map[string][]string{"live": {"open"}}, Vars: vars}
	return d
}

// constrained is a promotion of f in the one mode G under the constrains cs.
func constrained(f Family, cs ...Constrain) Promotion {
	return Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{f}, Constrains: cs}
}

func TestPromoteReadsAViewAsItsStates(t *testing.T) {
	// Setup
	locations := Family{Map: "locations", ID: "loc", Local: observedLocation(t)}
	p := modes(locations, Edge{Src: "G", Dst: "H", Event: ev("HALT", recv("loc")), Guard: mustParse(t, "@at(locations[loc], live)")})

	// Execute
	got, _, err := promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	halt := got.Edges[len(got.Edges)-1]
	if diff := cmp.Diff(mustParse(t, "@at(locations[loc], open)"), halt.Guard); diff != "" {
		t.Errorf("guard (-want +got):\n%s", diff)
	}
}

func TestPromoteRefusesAGlobalReadTheLocalDiagramDoesNotDeclare(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: observedLocation(t)}
	halt := func(guard string) Promotion {
		return modes(locations, Edge{Src: "G", Dst: "H", Event: ev("HALT", recv("loc")), Guard: mustParse(t, guard)})
	}
	cases := []struct {
		name string
		p    Promotion
		want string
	}{
		{"@at of a state", halt("@at(locations[loc], open)"), "G --> H : HALT: @at(locations[…], open) reads open, which the local diagram of locations does not declare as a view; it reads only its views (live); add ' CSDF-OBSERVE <view> = open to it"},
		{"a variable not declared", halt("${locations[loc].stock} = 0"), "G --> H : HALT: it reads locations[loc].stock, but the local diagram of locations does not declare stock observed; add ' CSDF-OBSERVE stock to it"},
		{"an instance as a whole", halt("${locations[loc]} = 0"), "it reads locations[loc], an instance as a whole"},
		{"in a sync", promotion(locations, Sync{Event: "MOVE", Guard: mustParse(t, "${locations[from].stock} = 0"), Targets: []Target{
			{Map: "locations", ID: "from", Event: "DEPART"}}}), "sync MOVE: it reads locations[from].stock"},
		{"in a constrain", constrained(locations, Constrain{Event: ev("OPEN", recv("loc")), Guard: mustParse(t, "@at(locations[loc], open)")}), "constrain OPEN: @at(locations[…], open)"},
		{"under an exists", halt("@exists(k : loc) (${locations[k].stock} = 0)"), "locations[…].stock"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, _, err := promote(c.p)

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}

func TestPromoteAcceptsAGlobalReadTheLocalDiagramDeclares(t *testing.T) {
	cases := []struct {
		name  string
		local *Diagram
		guard string
	}{
		{"existence", observedLocation(t), "@in(loc, locations) ∧ @notin(loc, locations)"},
		{"the domain", observedLocation(t), "@eq(locations, locations)"},
		{"a declared variable", observedLocation(t, "stock"), "${locations[loc].stock} = 0"},
		{"a state of a local diagram declaring nothing", location(t), "@at(locations[loc], open) ∧ ${locations[loc].stock} = 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			p := modes(Family{Map: "locations", ID: "loc", Local: c.local},
				Edge{Src: "G", Dst: "H", Event: ev("HALT", recv("loc")), Guard: mustParse(t, c.guard)})

			// Execute
			_, _, err := promote(p)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
