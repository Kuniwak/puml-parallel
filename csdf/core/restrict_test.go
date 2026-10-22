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

// safety is the safety specification of the traces of l: l with a τ from
// every state to STOP, so that it allows every refusal after each trace.
func safety(l *lts.LTS) *lts.LTS {
	out := lts.New(l.Init)
	stop := len(l.Trans)
	for src, byLabel := range l.Trans {
		if src >= stop {
			stop = src + 1
		}
		for label, dsts := range byLabel {
			for _, d := range dsts {
				out.Add(src, label, d)
			}
		}
	}
	for src := range l.Trans {
		out.Add(src, lts.Tau, stop)
	}
	return out
}

func TestRestrictKeepsTheKeptFamiliesWithTheOthersAsTheEnvironment(t *testing.T) {
	cases := []struct {
		name string
		door *Diagram
		// hidden are the events only the doors take.
		hidden func(string) bool
		sf     bool
	}{
		{
			// A door that is not there refuses SCRAP; the restriction,
			// whose doors are the environment, does not.
			name:   "the restriction offers what a left-out family refuses",
			door:   plainDoor(),
			hidden: func(string) bool { return false },
			sf:     false,
		},
		{
			// Hidden, a door's OPENS and SHUTS make it never stable, so
			// there is no failure to compare: divergence is outside the
			// stable failures model.
			name: "the events only a left-out family takes are hidden",
			door: door(),
			hidden: func(label string) bool {
				return strings.HasPrefix(label, "OPENS.") || strings.HasPrefix(label, "SHUTS.")
			},
			sf: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			p := fitting(c.door)
			full, _, err := Promote(p)
			if err != nil {
				t.Fatal(err)
			}
			env := stock(t, p.Families...)
			fullLTS, err := Expand(full, env)
			if err != nil {
				t.Fatal(err)
			}
			want := fittedKey("s1")

			// Execute
			restricted, warnings, err := Restrict(full, []string{"doors", "keys"}, []string{"keys"})

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			got, err := Expand(restricted, env)
			if err != nil {
				t.Fatal(err)
			}
			if !lts.StableFailuresEqual(got, want) {
				t.Errorf("the restriction is not the key alone, the doors offering anything")
			}
			hidden := lts.Hide(fullLTS, c.hidden)
			if spec := safety(got); !lts.StableFailuresRefines(spec, got) || !lts.StableFailuresRefines(spec, hidden) {
				t.Errorf("a safety specification the restriction refines is not refined by the full promotion with the doors' own events hidden")
			}
			if lts.StableFailuresRefines(got, hidden) != c.sf {
				t.Errorf("StableFailuresRefines = %v, want %v", !c.sf, c.sf)
			}
			if len(warnings) < 2 || !strings.Contains(warnings[0].Text, "spurious") || !strings.Contains(warnings[1].Text, "doors") {
				t.Errorf("warnings = %v, want what a restriction does not carry over, then the dropped clauses that read doors", warnings)
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
