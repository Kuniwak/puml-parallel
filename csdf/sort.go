package csdf

// Sort returns the canonical form of a diagram: the same meaning, printed in a
// fixed order so that two diagrams that mean the same thing print the same
// text. States are ordered by their id and transitions by CompareEdge; the
// start edge stays first and the end edge last, as Diagram.String always prints
// them. The declaration order of the state variables is authored meaning, so it
// is preserved. The result is a new diagram; the input is not modified.
//
// Source line numbers are dropped, because the canonical order is no longer the
// order of the source text; that is also what makes Diagram.String, which
// orders states by line first, fall back to ordering them by id.
func Sort(d *Diagram) *Diagram {
	states := make(map[StateID]State, len(d.States))
	for id, state := range d.States {
		state.Vars = append([]StateVar{}, state.Vars...)
		state.Line = 0
		states[id] = state
	}

	edges := make([]Edge, 0, len(d.Edges))
	for _, edge := range d.Edges {
		edge.Line = 0
		edges = append(edges, edge)
	}
	SortEdges(edges)

	startEdge := d.StartEdge
	startEdge.Line = 0

	var endEdge *EndEdge
	if d.EndEdge != nil {
		copied := *d.EndEdge
		copied.Line = 0
		endEdge = &copied
	}

	return &Diagram{
		States:    states,
		StartEdge: startEdge,
		Edges:     edges,
		EndEdge:   endEdge,
	}
}
