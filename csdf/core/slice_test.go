package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// chain is a --x--> b --y--> c --z--> d, starting at a.
func chain() *Diagram {
	return &Diagram{
		States: []string{"a", "b", "c", "d"},
		Init:   "a",
		Edges: []Edge{
			{Src: "a", Dst: "b", Event: Event{Channel: "x"}},
			{Src: "b", Dst: "c", Event: Event{Channel: "y"}},
			{Src: "c", Dst: "d", Event: Event{Channel: "z"}},
		},
	}
}

func TestSliceKeepsTheStatesWithinTheContext(t *testing.T) {
	cases := map[string]struct {
		states  []string
		context int
		want    []string
	}{
		"context 0 keeps only the given states (lower boundary value)": {[]string{"c"}, 0, []string{"c"}},
		"context 1 follows edges both ways (representative value)":     {[]string{"c"}, 1, []string{"b", "c", "d"}},
		"two given states join their neighbourhoods":                   {[]string{"a", "d"}, 1, []string{"a", "b", "c", "d"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Slice(chain(), c.states, c.context)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got.Diagram.States); diff != "" {
				t.Errorf("States (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSliceRefusesAnUnknownState(t *testing.T) {
	// Execute
	_, err := Slice(chain(), []string{"b", "nowhere"}, 1)

	// Assert
	if err == nil || err.Error() != `core.Slice: unknown state "nowhere"` {
		t.Errorf("want the unknown state named, got %v", err)
	}
}

func TestSliceKeepsTheEdgesInsideAndCutsTheEdgesLeaving(t *testing.T) {
	// Setup
	d := chain()

	// Execute
	got, err := Slice(d, []string{"b"}, 0)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Diagram.Edges) != 0 {
		t.Errorf("want no edge inside, got %v", got.Diagram.Edges)
	}
	if diff := cmp.Diff([]Edge{d.Edges[0], d.Edges[1]}, got.Cut); diff != "" {
		t.Errorf("Cut (-want +got):\n%s", diff)
	}
}

func TestSliceKeepsWhatTheKeptStatesAndEdgesNeed(t *testing.T) {
	// Setup
	d := chain()
	d.Labels = map[string]string{"b": "Bee", "d": "Dee"}
	d.Vars = map[string][]string{"b": {"n"}, "d": {"m"}}
	d.Channels = map[string][]types.Type{"x": nil, "y": nil, "z": nil}
	d.Ends = []End{{Src: "b"}, {Src: "d"}}
	d.Assumes = []Assumption{{ID: "fair", Text: "fairness"}}

	// Execute
	got, err := Slice(d, []string{"b"}, 1)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := &Diagram{
		States:   []string{"a", "b", "c"},
		Labels:   map[string]string{"b": "Bee"},
		Vars:     map[string][]string{"b": {"n"}},
		Channels: map[string][]types.Type{"x": nil, "y": nil},
		Init:     "a",
		Edges:    []Edge{d.Edges[0], d.Edges[1]},
		Ends:     []End{{Src: "b"}},
		Assumes:  d.Assumes,
	}
	if diff := cmp.Diff(want, got.Diagram); diff != "" {
		t.Errorf("Diagram (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]Edge{d.Edges[2]}, got.Cut); diff != "" {
		t.Errorf("Cut (-want +got):\n%s", diff)
	}
}

func TestSliceLeavesTheStartOutWhenTheInitialStateIsCutOff(t *testing.T) {
	// Execute
	got, err := Slice(chain(), []string{"d"}, 1)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got.Diagram.Init != "" {
		t.Errorf("want no initial state, got %q", got.Diagram.Init)
	}
}
