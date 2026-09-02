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
	out := d.Clone()

	for id, state := range out.States {
		state.Line = 0
		out.States[id] = state
	}
	for i := range out.Edges {
		out.Edges[i].Line = 0
	}
	out.StartEdge.Line = 0
	if out.EndEdge != nil {
		out.EndEdge.Line = 0
	}

	SortEdges(out.Edges)
	return out
}
