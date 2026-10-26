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
	for _, e := range d.Edges {
		if e.Src != s || e.Event.Channel != Tau {
			continue
		}
		if _, ok := d.TauPath(e.Dst, s, everyEdge); ok {
			return true
		}
	}
	return false
}

func everyEdge(Edge) bool { return true }

// TauPath is a shortest path of states from from to to over the tau edges
// of d that keep accepts, both ends included, the edges taken in the order
// they are written. It reports false when there is none.
func (d *Diagram) TauPath(from, to string, keep func(Edge) bool) ([]string, bool) {
	parent := map[string]string{from: ""}
	todo := []string{from}
	for len(todo) > 0 {
		s := todo[0]
		todo = todo[1:]
		if s == to {
			var path []string
			for t := to; t != from; t = parent[t] {
				path = append(path, t)
			}
			path = append(path, from)
			slices.Reverse(path)
			return path, true
		}
		for _, e := range d.Edges {
			if e.Src != s || e.Event.Channel != Tau || !keep(e) {
				continue
			}
			if _, seen := parent[e.Dst]; !seen {
				parent[e.Dst] = s
				todo = append(todo, e.Dst)
			}
		}
	}
	return nil, false
}
