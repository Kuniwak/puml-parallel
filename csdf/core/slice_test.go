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

// decoratedChain is chain with labels, variables, channels, end edges and
// an assumption.
func decoratedChain() *Diagram {
	d := chain()
	d.Labels = map[string]string{"b": "Bee", "d": "Dee"}
	d.Vars = map[string][]string{"b": {"n"}, "d": {"m"}}
	d.Channels = map[string][]types.Type{"x": nil, "y": nil, "z": nil}
	d.Ends = []End{{Src: "b"}, {Src: "d"}}
	d.Assumes = []Assumption{{ID: "fair", Text: "fairness"}}
	return d
}

func TestSlice(t *testing.T) {
	x, y, z := chain().Edges[0], chain().Edges[1], chain().Edges[2]
	redrawn := func(e Edge, src, dst string) Edge {
		e.Src, e.Dst = src, dst
		return e
	}
	cases := map[string]struct {
		d               *Diagram
		states          []string
		context         int
		want            *Diagram
		wantCut         []Edge
		wantOutside     []Placeholder
		wantOutsideInit string
	}{
		"context 0 keeps only the given state and cuts the edges at it (lower boundary value)": {
			d: chain(), states: []string{"b"}, context: 0,
			want:            &Diagram{States: []string{"b"}},
			wantCut:         []Edge{redrawn(x, "a_cut", "b"), redrawn(y, "b", "c_cut")},
			wantOutside:     []Placeholder{{ID: "a_cut", Label: "a …"}, {ID: "c_cut", Label: "c …"}},
			wantOutsideInit: "a_cut",
		},
		"context 1 follows edges both ways (representative value)": {
			d: chain(), states: []string{"c"}, context: 1,
			want:            &Diagram{States: []string{"b", "c", "d"}, Edges: []Edge{y, z}},
			wantCut:         []Edge{redrawn(x, "a_cut", "b")},
			wantOutside:     []Placeholder{{ID: "a_cut", Label: "a …"}},
			wantOutsideInit: "a_cut",
		},
		"a placeholder's ID collides with no name of the diagram": {
			d: &Diagram{
				States: []string{"a", "a_cut"},
				Init:   "a_cut",
				Edges:  []Edge{{Src: "a_cut", Dst: "a", Event: Event{Channel: "w"}}},
			},
			states: []string{"a"}, context: 0,
			want:            &Diagram{States: []string{"a"}},
			wantCut:         []Edge{{Src: "a_cut_cut_cut", Dst: "a", Event: Event{Channel: "w"}}},
			wantOutside:     []Placeholder{{ID: "a_cut_cut_cut", Label: "a_cut …"}},
			wantOutsideInit: "a_cut_cut_cut",
		},
		"two given states join their neighbourhoods": {
			d: chain(), states: []string{"a", "d"}, context: 1,
			want: &Diagram{States: []string{"a", "b", "c", "d"}, Init: "a", Edges: []Edge{x, y, z}},
		},
		"what the kept states and edges need is kept": {
			d: decoratedChain(), states: []string{"b"}, context: 1,
			want: &Diagram{
				States:   []string{"a", "b", "c"},
				Labels:   map[string]string{"b": "Bee"},
				Vars:     map[string][]string{"b": {"n"}},
				Channels: map[string][]types.Type{"x": nil, "y": nil},
				Init:     "a",
				Edges:    []Edge{x, y},
				Ends:     []End{{Src: "b"}},
				Assumes:  []Assumption{{ID: "fair", Text: "fairness"}},
			},
			wantCut:     []Edge{redrawn(z, "c", "d_cut")},
			wantOutside: []Placeholder{{ID: "d_cut", Label: "Dee …"}},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := c.d

			// Execute
			got, err := Slice(d, c.states, c.context)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got.Diagram); diff != "" {
				t.Errorf("Diagram (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.wantCut, got.Cut); diff != "" {
				t.Errorf("Cut (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.wantOutside, got.Outside); diff != "" {
				t.Errorf("Outside (-want +got):\n%s", diff)
			}
			if got.OutsideInit != c.wantOutsideInit {
				t.Errorf("OutsideInit: want %q, got %q", c.wantOutsideInit, got.OutsideInit)
			}
		})
	}
}

func TestSliceRefusesAnUnknownState(t *testing.T) {
	// Setup
	d := chain()

	// Execute
	_, err := Slice(d, []string{"b", "nowhere"}, 1)

	// Assert
	if err == nil || err.Error() != `core.Slice: unknown state "nowhere"` {
		t.Errorf("want the unknown state named, got %v", err)
	}
}
