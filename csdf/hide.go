package csdf

// Hide renames every occurrence of the given events to the internal event τ
// (CSP hiding, P \ A). The result is a new diagram; the input is not modified.
// Events not occurring in the diagram are ignored, and hiding τ itself is a
// no-op because τ is already internal.
func Hide(d *Diagram, events []Event) *Diagram {
	hidden := make(map[Event]struct{}, len(events))
	for _, event := range events {
		hidden[event] = struct{}{}
	}

	edges := make([]Edge, 0, len(d.Edges))
	for _, edge := range d.Edges {
		if _, ok := hidden[edge.Event]; ok {
			edge.Event = Tau
		}
		edges = append(edges, edge)
	}

	states := make(map[StateID]State, len(d.States))
	for id, state := range d.States {
		states[id] = state
	}

	return &Diagram{
		States:    states,
		StartEdge: d.StartEdge,
		Edges:     edges,
		EndEdge:   d.EndEdge,
	}
}
