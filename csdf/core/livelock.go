package core

// TauCycleReachable reports whether the graph of the states of d, every edge
// taken whatever its clauses, reaches a cycle of tau edges from the start.
// When it does not, d is livelock free whatever its clauses say.
func (d *Diagram) TauCycleReachable() bool {
	for s := range d.reachedStates() {
		if d.onTauCycle(s) {
			return true
		}
	}
	return false
}

func (d *Diagram) reachedStates() map[string]bool {
	reached := map[string]bool{d.Init: true}
	todo := []string{d.Init}
	for len(todo) > 0 {
		s := todo[0]
		todo = todo[1:]
		for _, e := range d.Edges {
			if e.Src == s && !reached[e.Dst] {
				reached[e.Dst] = true
				todo = append(todo, e.Dst)
			}
		}
	}
	return reached
}

// onTauCycle reports whether s reaches itself by tau edges alone.
func (d *Diagram) onTauCycle(s string) bool {
	seen := map[string]bool{}
	todo := []string{s}
	for len(todo) > 0 {
		t := todo[0]
		todo = todo[1:]
		for _, e := range d.Edges {
			if e.Src != t || e.Event.Channel != Tau {
				continue
			}
			if e.Dst == s {
				return true
			}
			if !seen[e.Dst] {
				seen[e.Dst] = true
				todo = append(todo, e.Dst)
			}
		}
	}
	return false
}
