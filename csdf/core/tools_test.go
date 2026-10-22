package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWarningString(t *testing.T) {
	cases := []struct {
		name string
		w    Warning
		want string
	}{
		{"a warning about an event of a map", Warning{Map: "doors", Event: "OPEN", Text: "t"}, "warning: doors OPEN: t"},
		{"an info about a map", Warning{Level: Info, Map: "doors", Text: "t"}, "info: doors: t"},
		{"a note about the whole", Warning{Level: Note, Text: Approximation}, "note: " + Approximation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got := c.w.String()

			// Assert
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestRestrictSaysWhatItsResultMeansAsANote(t *testing.T) {
	// Setup
	d := &Diagram{States: []string{"G"}, Init: "G", Vars: map[string][]string{"G": {"doors"}}}

	// Execute
	_, warnings, err := Restrict(d, []string{"doors"}, []string{"doors"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(Warning{Level: Note, Text: Approximation}, warnings[0]); diff != "" {
		t.Errorf("first warning (-want +got):\n%s", diff)
	}
}

func TestPromoteOnlyIsPromoteThenRestrict(t *testing.T) {
	// Setup
	p := fitting(plainDoor())
	full, fw, err := Promote(p)
	if err != nil {
		t.Fatal(err)
	}
	want, rw, err := Restrict(full, []string{"doors", "keys"}, []string{"keys"})
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, warnings, err := PromoteOnly(p, []string{"keys"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagram (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(append(fw, rw...), warnings); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
}

func TestComposeAllFoldsLeftAndDropsWhatNoEdgeReaches(t *testing.T) {
	// Setup: a and c step on go together; b of the first is never reached.
	l := &Diagram{States: []string{"a", "b"}, Init: "a", Edges: []Edge{{Src: "a", Dst: "a", Event: ev("go")}}}
	m := &Diagram{States: []string{"c"}, Init: "c", Edges: []Edge{{Src: "c", Dst: "c", Event: ev("go")}}}
	r := &Diagram{States: []string{"d"}, Init: "d"}

	// Execute
	got, err := ComposeAll([]*Diagram{l, m, r}, []string{"go"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"a_c_d"}, got.States); diff != "" {
		t.Errorf("States (-want +got):\n%s", diff)
	}
}

func TestConstructorsKeepTheTitleAndTheLabels(t *testing.T) {
	d := &Diagram{Name: "N", Labels: map[string]string{"G": "稼働中"}, States: []string{"G"}, Init: "G",
		Vars: map[string][]string{"G": {"doors"}}, Edges: []Edge{{Src: "G", Dst: "G", Event: ev("go")}}}
	cases := []struct {
		name string
		op   func() (*Diagram, error)
	}{
		{"Hide", func() (*Diagram, error) { return Hide(d, []string{"go"}) }},
		{"Restrict", func() (*Diagram, error) {
			r, _, err := Restrict(d, []string{"doors"}, []string{"doors"})
			return r, err
		}},
		{"Reachable", func() (*Diagram, error) { return Reachable(d), nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got, err := c.op()

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "N" || got.Labels["G"] != "稼働中" {
				t.Errorf("Name %q, Labels %v", got.Name, got.Labels)
			}
		})
	}
}
