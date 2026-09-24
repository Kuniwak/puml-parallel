package csdf

// Graph is the edges of a diagram indexed by source, each list in canonical
// order, so that a walk over it comes out the same for the same diagram. It is
// made only by NewGraph, which is what guarantees the order.
type Graph struct {
	out map[StateID][]Edge
}

// NewGraph indexes the edges of d.
func NewGraph(d *Diagram) Graph {
	out := make(map[StateID][]Edge)
	for _, e := range d.Edges {
		out[e.Src] = append(out[e.Src], e)
	}
	for s := range out {
		SortEdges(out[s])
	}
	return Graph{out: out}
}

// Out returns the edges leaving s, in canonical order. The slice is the
// graph's own; it must not be modified.
func (g Graph) Out(s StateID) []Edge { return g.out[s] }

// Taus returns the tau edges leaving s, in canonical order, or nil when there
// are none.
func (g Graph) Taus(s StateID) []Edge {
	var taus []Edge
	for _, e := range g.out[s] {
		if e.Event == Tau {
			taus = append(taus, e)
		}
	}
	return taus
}

// Reachable returns the states reachable from start over all edges, start
// included, in the order a breadth-first walk meets them.
func (g Graph) Reachable(start StateID) []StateID {
	order := []StateID{start}
	seen := map[StateID]struct{}{start: {}}
	for i := 0; i < len(order); i++ {
		for _, e := range g.out[order[i]] {
			if _, ok := seen[e.Dst]; !ok {
				seen[e.Dst] = struct{}{}
				order = append(order, e.Dst)
			}
		}
	}
	return order
}

// TauClosure returns the states reachable from set via zero or more tau edges,
// the states of set included.
func (g Graph) TauClosure(set map[StateID]struct{}) map[StateID]struct{} {
	closure := make(map[StateID]struct{}, len(set))
	queue := make([]StateID, 0, len(set))
	for s := range set {
		closure[s] = struct{}{}
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, e := range g.Taus(s) {
			if _, ok := closure[e.Dst]; !ok {
				closure[e.Dst] = struct{}{}
				queue = append(queue, e.Dst)
			}
		}
	}
	return closure
}
