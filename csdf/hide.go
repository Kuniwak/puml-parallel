package csdf

// Hide renames every occurrence of the given events to the internal event τ
// (CSP hiding, P \ A). The result is a new diagram; the input is not modified.
// Events not occurring in the diagram are ignored, and hiding τ itself is a
// no-op because τ is already internal. The order of the transitions is that of
// the input, since hiding only renames events.
func Hide(d *Diagram, events []Event) *Diagram {
	hidden := make(map[Event]struct{}, len(events))
	for _, event := range events {
		hidden[event] = struct{}{}
	}

	out := d.Clone()
	for i, edge := range out.Edges {
		if _, ok := hidden[edge.Event]; ok {
			out.Edges[i].Event = Tau
		}
	}
	return out
}
