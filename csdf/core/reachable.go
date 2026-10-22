package core

import "slices"

// Reachable is d without the states no chain of edges leads to from Init,
// and without their edges, reading no clause (docs/CLAUSE_DESIGN.md, a
// change of representation): a run never enters such a state, so
// ⟦Reachable(d)⟧ =SF ⟦d⟧, and the diagram has fewer states to read and to
// walk. End edges and labels of the states it drops go with them.
func Reachable(d *Diagram) *Diagram {
	reached := map[string]bool{d.Init: true}
	for queue := []string{d.Init}; len(queue) > 0; queue = queue[1:] {
		for _, e := range d.Edges {
			if e.Src == queue[0] && !reached[e.Dst] {
				reached[e.Dst] = true
				queue = append(queue, e.Dst)
			}
		}
	}
	out := *d
	out.States = slices.DeleteFunc(slices.Clone(d.States), func(s string) bool { return !reached[s] })
	out.Edges = slices.DeleteFunc(slices.Clone(d.Edges), func(e Edge) bool { return !reached[e.Src] })
	out.Ends = slices.DeleteFunc(slices.Clone(d.Ends), func(e End) bool { return !reached[e.Src] })
	out.Vars = map[string][]string{}
	for s, vs := range d.Vars {
		if reached[s] {
			out.Vars[s] = vs
		}
	}
	if d.Labels != nil {
		out.Labels = map[string]string{}
		for s, l := range d.Labels {
			if reached[s] {
				out.Labels[s] = l
			}
		}
	}
	return &out
}
