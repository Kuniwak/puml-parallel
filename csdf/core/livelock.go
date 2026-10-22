package core

import "slices"

// TauCycleReachable reports whether the graph of the states of d, every edge
// taken whatever its clauses, reaches a cycle of tau edges from the start.
// When it does not, d is livelock free whatever its clauses say.
func (d *Diagram) TauCycleReachable() bool { return len(d.TauCycleStates()) > 0 }

// TauCycleStates are the states reachable from the start that lie on a
// cycle of tau edges, every edge taken whatever its clauses, sorted.
func (d *Diagram) TauCycleStates() []string {
	var on []string
	for s := range d.reachedStates() {
		if d.onTauCycle(s) {
			on = append(on, s)
		}
	}
	slices.Sort(on)
	return on
}

// ReachableStates are the states reachable from the start, every edge taken
// whatever its clauses, in the order of the states of d.
func (d *Diagram) ReachableStates() []string {
	reached := d.reachedStates()
	var out []string
	for _, s := range d.States {
		if reached[s] {
			out = append(out, s)
		}
	}
	return out
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
