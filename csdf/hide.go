package csdf

// Hide renames every event on the given channels to the internal event τ
// (CSP hiding, P \ {|c|}). A field of a hidden event becomes a value the τ
// chooses: c?x hidden is ⊓ over x of τ, and the clauses still read x by its
// name. The result is a new diagram; the input is not modified. Channels not
// occurring in the diagram are ignored, and hiding τ itself is a no-op because
// τ is already internal. The order of the transitions is that of the input,
// since hiding only renames events.
func Hide(d *Diagram, channels []string) *Diagram {
	hidden := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		hidden[channel] = struct{}{}
	}

	out := d.Clone()
	for i, edge := range out.Edges {
		if _, ok := hidden[edge.Event.Channel()]; ok {
			out.Edges[i].Event = Tau
		}
	}
	return out
}
