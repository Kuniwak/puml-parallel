package csdf

// Outgoing indexes the edges of d by source, each list in canonical order, so
// that a walk over them comes out the same for the same diagram.
func Outgoing(d *Diagram) map[StateID][]Edge {
	out := make(map[StateID][]Edge)
	for _, e := range d.Edges {
		out[e.Src] = append(out[e.Src], e)
	}
	for s := range out {
		SortEdges(out[s])
	}
	return out
}

// TauEdges returns the tau edges among edges, in their order, or nil when
// there are none.
func TauEdges(edges []Edge) []Edge {
	var taus []Edge
	for _, e := range edges {
		if e.Event == Tau {
			taus = append(taus, e)
		}
	}
	return taus
}

// Reachable returns the states reachable from start over all edges of out,
// start included, in the order a breadth-first walk meets them when it follows
// each list of out in order.
func Reachable(start StateID, out map[StateID][]Edge) []StateID {
	order := []StateID{start}
	seen := map[StateID]struct{}{start: {}}
	for i := 0; i < len(order); i++ {
		for _, e := range out[order[i]] {
			if _, ok := seen[e.Dst]; !ok {
				seen[e.Dst] = struct{}{}
				order = append(order, e.Dst)
			}
		}
	}
	return order
}
