package core

import (
	"slices"
	"testing"
)

func TestTauPath(t *testing.T) {
	tau := func(src, dst string, line int) Edge {
		return Edge{Src: src, Dst: dst, Event: Event{Channel: Tau}, Line: line}
	}
	d := &Diagram{
		States: []string{"a", "b", "c"},
		Init:   "a",
		Edges: []Edge{
			tau("a", "b", 1),
			tau("b", "a", 2),
			tau("b", "c", 3),
			tau("c", "a", 4),
			{Src: "a", Dst: "c", Event: Event{Channel: "x"}, Line: 5},
		},
	}
	cases := map[string]struct {
		from, to string
		keep     func(Edge) bool
		want     []string
		wantOK   bool
	}{
		"the shortest path over every tau edge": {
			from: "a", to: "a", keep: everyEdge, want: []string{"a"}, wantOK: true,
		},
		"the path from b back to a takes the first written edge": {
			from: "b", to: "a", keep: everyEdge, want: []string{"b", "a"}, wantOK: true,
		},
		"an edge keep refuses is not taken": {
			from: "b", to: "a", keep: func(e Edge) bool { return e.Line != 2 },
			want: []string{"b", "c", "a"}, wantOK: true,
		},
		"an observable edge is never taken": {
			from: "a", to: "c", keep: func(e Edge) bool { return e.Line != 3 },
			wantOK: false,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := d.TauPath(c.from, c.to, c.keep)
			if ok != c.wantOK || !slices.Equal(got, c.want) {
				t.Errorf("TauPath(%q, %q) = %v, %v; want %v, %v", c.from, c.to, got, ok, c.want, c.wantOK)
			}
		})
	}
}
