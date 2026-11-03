package core

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// Sliced is the part of a diagram around some of its states. Diagram holds
// the states within the context, the edges between them, the end edges from
// them, the channels those edges use and every assumption; its Init is empty
// when the initial state is outside. Outside are placeholders for the
// states outside that an edge or the start edge reaches from inside,
// ordered by ID; Cut are the edges with one end inside and the other
// outside, their outside end redrawn to its placeholder; OutsideInit, with
// OutsideInitPost, is the placeholder of the initial state when it is
// outside, and empty otherwise. A slice means something else than the
// diagram it was cut from - what is reachable changes - so it is for
// reading only.
type Sliced struct {
	Diagram         *Diagram
	Outside         []Placeholder
	Cut             []Edge
	OutsideInit     string
	OutsideInitPost clause.And
}

// Placeholder stands for a state outside a slice: its ID is the state's ID
// with a suffix that makes it collide with no name of the diagram, its
// label the state's label followed by " …".
type Placeholder struct {
	ID, Label string
}

// Slice is the part of d within context edges of any of states, the edges
// followed in either direction.
func Slice(d *Diagram, states []string, context int) (*Sliced, error) {
	for _, s := range states {
		if !slices.Contains(d.States, s) {
			return nil, fmt.Errorf("core.Slice: %w", fmt.Errorf("unknown state %q", s))
		}
	}
	in := neighbourhood(d, states, context)
	out := &Diagram{Name: d.Name, Types: d.Types, Assumes: d.Assumes}
	for _, s := range d.States {
		if !in[s] {
			continue
		}
		out.States = append(out.States, s)
		if l, ok := d.Labels[s]; ok {
			if out.Labels == nil {
				out.Labels = map[string]string{}
			}
			out.Labels[s] = l
		}
		if vs, ok := d.Vars[s]; ok {
			if out.Vars == nil {
				out.Vars = map[string][]string{}
			}
			out.Vars[s] = vs
		}
	}
	if in[d.Init] {
		out.Init, out.InitPost, out.InitLine = d.Init, d.InitPost, d.InitLine
	}
	suffix := freshSuffix(d)
	outside := map[string]bool{}
	placeholder := func(s string) string {
		if in[s] {
			return s
		}
		outside[s] = true
		return s + suffix
	}
	var cut []Edge
	for _, e := range d.Edges {
		switch {
		case in[e.Src] && in[e.Dst]:
			out.Edges = append(out.Edges, e)
			if fields, ok := d.Channels[e.Event.Channel]; ok {
				if out.Channels == nil {
					out.Channels = map[string][]types.Type{}
				}
				out.Channels[e.Event.Channel] = fields
			}
		case in[e.Src] || in[e.Dst]:
			e.Src, e.Dst = placeholder(e.Src), placeholder(e.Dst)
			cut = append(cut, e)
		}
	}
	for _, e := range d.Ends {
		if in[e.Src] {
			out.Ends = append(out.Ends, e)
		}
	}
	sliced := &Sliced{Diagram: out, Cut: cut}
	if !in[d.Init] {
		sliced.OutsideInit, sliced.OutsideInitPost = placeholder(d.Init), d.InitPost
	}
	for _, s := range slices.Sorted(maps.Keys(outside)) {
		sliced.Outside = append(sliced.Outside, Placeholder{ID: s + suffix, Label: d.label(s) + " …"})
	}
	return sliced, nil
}

// freshSuffix is "_cut", repeated until no state of d, with it appended, is
// a name d already has: a state, an assumption or the channel note.
func freshSuffix(d *Diagram) string {
	names := map[string]bool{"channels": true}
	for _, s := range d.States {
		names[s] = true
	}
	for _, a := range d.Assumes {
		names[a.ID] = true
	}
	suffix := "_cut"
	for slices.ContainsFunc(d.States, func(s string) bool { return names[s+suffix] }) {
		suffix += "_cut"
	}
	return suffix
}

// neighbourhood is the set of the states within context edges of any of
// states, the edges followed in either direction.
func neighbourhood(d *Diagram, states []string, context int) map[string]bool {
	adjacent := map[string][]string{}
	for _, e := range d.Edges {
		adjacent[e.Src] = append(adjacent[e.Src], e.Dst)
		adjacent[e.Dst] = append(adjacent[e.Dst], e.Src)
	}
	in := map[string]bool{}
	var frontier []string
	for _, s := range states {
		if !in[s] {
			in[s] = true
			frontier = append(frontier, s)
		}
	}
	for range context {
		var next []string
		for _, s := range frontier {
			for _, t := range adjacent[s] {
				if !in[t] {
					in[t] = true
					next = append(next, t)
				}
			}
		}
		frontier = next
	}
	return in
}
