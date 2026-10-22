package core

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

// plainDoor: NEW creates it shut, and GONE deletes it.
func plainDoor() *Diagram {
	return &Diagram{
		States: []string{"none", "shut"},
		Init:   "none",
		Edges: []Edge{
			{Src: "none", Dst: "shut", Event: ev("NEW")},
			{Src: "shut", Dst: "none", Event: ev("GONE")},
		},
	}
}

// fitting is doors created and deleted together with a key, INSTALL and
// SCRAP, the doors running door.
func fitting(door *Diagram) Promotion {
	return Promotion{Modes: []string{"G"}, Init: "G",
		Families: []Family{{Map: "doors", ID: "loc", Local: door}, {Map: "keys", ID: "s", Local: key()}},
		Syncs: []Sync{
			{Event: "INSTALL", Targets: []Target{{Map: "doors", ID: "loc", Event: "NEW"}, {Map: "keys", ID: "s", Event: "CUT"}}},
			{Event: "SCRAP", Targets: []Target{{Map: "doors", ID: "loc", Event: "GONE"}, {Map: "keys", ID: "s", Event: "LOST"}}},
		}}
}

func TestRestrictKeepsTheKeptFamiliesWithTheOthersAsTheEnvironment(t *testing.T) {
	locations := Family{Map: "locations", ID: "loc", Local: location(t)}
	shipments := Family{Map: "shipments", ID: "s", Local: shipment(t)}
	moving := Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{locations, shipments}, Syncs: []Sync{{Event: "MOVE", Targets: []Target{
		{Map: "shipments", ID: "s", Event: "MOVE"},
		{Map: "locations", ID: "from", Event: "DEPART"},
		{Map: "locations", ID: "to", Event: "ARRIVE"},
	}}}}
	onlyLocations := func(label string) bool {
		return strings.HasPrefix(label, "OPEN.") || strings.HasPrefix(label, "CLOSE.")
	}
	cases := []struct {
		name string
		p    Promotion
		keep []string
		// hidden are the events only the left-out families take.
		hidden func(string) bool
		// want is the restriction written by hand, if it is written.
		want        *lts.LTS
		refinesFull bool
		dropped     string
	}{
		{
			// A door that is not there refuses SCRAP; the restriction,
			// whose doors are the environment, does not.
			name:        "the restriction offers what a left-out family refuses",
			p:           fitting(plainDoor()),
			keep:        []string{"keys"},
			hidden:      func(string) bool { return false },
			want:        fittedKey("s1"),
			refinesFull: false,
			dropped:     "doors",
		},
		{
			// Hidden, a door's OPENS and SHUTS make it never stable, so
			// there is no failure to compare: divergence is outside the
			// stable failures model.
			name: "the events only a left-out family takes are hidden",
			p:    fitting(door()),
			keep: []string{"keys"},
			hidden: func(label string) bool {
				return strings.HasPrefix(label, "OPENS.") || strings.HasPrefix(label, "SHUTS.")
			},
			want:        fittedKey("s1"),
			refinesFull: true,
			dropped:     "doors",
		},
		{
			// MOVE chains two moves of locations through ∃, one clause
			// that holds the shipments' move too: dropped whole, it
			// leaves shipments free there.
			name:    "chained moves of a left-out map free the kept one",
			p:       moving,
			keep:    []string{"shipments"},
			hidden:  onlyLocations,
			dropped: "locations",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			full, _, err := Promote(c.p)
			if err != nil {
				t.Fatal(err)
			}
			env := stock(t, c.p.Families...)
			fullLTS, err := Expand(full, env)
			if err != nil {
				t.Fatal(err)
			}
			var maps []string
			for _, f := range c.p.Families {
				maps = append(maps, f.Map)
			}
			hidden := lts.Hide(fullLTS, c.hidden)

			// Execute
			restricted, warnings, err := Restrict(full, maps, c.keep)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Expand(restricted, env)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if c.want != nil && !lts.StableFailuresEqual(got, c.want) {
				t.Errorf("the restriction is not the kept families alone, the others offering anything")
			}
			// Every safety specification the restriction refines, the
			// full promotion refines with hidden hidden: its traces are
			// the restriction's.
			if !lts.TracesRefines(got, hidden) {
				t.Errorf("the full promotion, hidden, has a trace the restriction has not")
			}
			if lts.StableFailuresRefines(got, hidden) != c.refinesFull {
				t.Errorf("StableFailuresRefines = %v, want %v", !c.refinesFull, c.refinesFull)
			}
			if len(warnings) < 2 || !strings.Contains(warnings[0].Text, "spurious") || !strings.Contains(warnings[1].Text, c.dropped) {
				t.Errorf("warnings = %v, want what a restriction does not carry over, then the dropped clauses that read %s", warnings, c.dropped)
			}
		})
	}
}

func TestRestrictRefuses(t *testing.T) {
	cases := []struct {
		name string
		keep []string
		want string
	}{
		{"keeping nothing", nil, "keeps no family"},
		{"keeping a map that is not promoted", []string{"locks"}, "locks"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			full, _, err := Promote(fitting(plainDoor()))
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			_, _, err = Restrict(full, []string{"doors", "keys"}, c.keep)

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}

func TestRestrictReportsEveryClauseItDrops(t *testing.T) {
	// Setup
	full, _, err := Promote(fitting(plainDoor()))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, warnings, err := Restrict(full, []string{"doors", "keys"}, []string{"keys"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, w := range warnings {
		texts = append(texts, w.Event+": "+w.Text)
	}
	all := strings.Join(texts, "\n")
	for _, want := range []string{"[*]: dropped, as it reads a left-out map: @empty(doors')", "INSTALL: dropped, as it reads a left-out map: @at(doors'[loc], shut)"} {
		if !strings.Contains(all, want) {
			t.Errorf("warnings lack %q:\n%s", want, all)
		}
	}
}
