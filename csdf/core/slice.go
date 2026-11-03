package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// Sliced is the part of a diagram around some of its states. Diagram holds
// the states within the context, the edges between them, the end edges from
// them, the channels those edges use and every assumption; its Init is empty
// when the initial state is outside. Cut are the edges with one end inside
// and the other outside. From is the diagram the slice was cut from. A slice means something else than the diagram it
// was cut from - what is reachable changes - so it is for reading only.
type Sliced struct {
	Diagram *Diagram
	Cut     []Edge
	From    *Diagram
}

// Slice is the part of d within context edges of any of states, the edges
// followed in either direction.
func Slice(d *Diagram, states []string, context int) (*Sliced, error) {
	for _, s := range states {
		if !slices.Contains(d.States, s) {
			return nil, fmt.Errorf("core.Slice: unknown state %q", s)
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
			cut = append(cut, e)
		}
	}
	for _, e := range d.Ends {
		if in[e.Src] {
			out.Ends = append(out.Ends, e)
		}
	}
	return &Sliced{Diagram: out, Cut: cut, From: d}, nil
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
