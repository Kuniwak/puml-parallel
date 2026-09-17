package tracechk

import (
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Path is one way the diagram performs a trace: the edges it takes in order,
// tau edges included, from the start state to the state after the last event.
type Path []csdf.Edge

// Rejection says where a trace stops being one, even with every guard true.
type Rejection struct {
	// Index is the 0-based position in the trace of the event the diagram
	// cannot perform.
	Index int
	Event csdf.Event
	// States are the states the diagram may be in just before that event,
	// tau-closed and sorted.
	States []csdf.StateID
	// Enabled are the visible events some of those states can perform, sorted.
	Enabled []csdf.Event
}

// Check reports whether trace is a trace of d when every guard is taken as
// true. When it is, it returns every path performing it; a path never repeats a
// state within one run of tau edges, so a tau cycle yields finitely many paths.
// When it is not, it returns where the trace was rejected.
//
// Guards and postconditions are natural language and are not evaluated here:
// an accepted trace is a trace of the diagram only if some returned path has
// satisfiable predicates, which is the caller's obligation to state.
func Check(d *csdf.Diagram, trace []csdf.Event) ([]Path, *Rejection) {
	out := outgoing(d)

	states := tauClosure(map[csdf.StateID]struct{}{d.StartEdge.Dst: {}}, out)
	for i, event := range trace {
		next := make(map[csdf.StateID]struct{})
		for s := range states {
			for _, e := range out[s] {
				if e.Event == event {
					next[e.Dst] = struct{}{}
				}
			}
		}
		if len(next) == 0 {
			return nil, &Rejection{
				Index:   i,
				Event:   event,
				States:  sortedStates(states),
				Enabled: enabledEvents(states, out),
			}
		}
		states = tauClosure(next, out)
	}

	var paths []Path
	var extend func(s csdf.StateID, i int, path Path, tauSeen map[csdf.StateID]struct{})
	extend = func(s csdf.StateID, i int, path Path, tauSeen map[csdf.StateID]struct{}) {
		if i == len(trace) {
			paths = append(paths, slices.Clone(path))
			return
		}
		for _, e := range out[s] {
			switch {
			case e.Event == trace[i]:
				extend(e.Dst, i+1, append(path, e), map[csdf.StateID]struct{}{e.Dst: {}})
			case e.Event == csdf.Tau:
				if _, seen := tauSeen[e.Dst]; seen {
					continue
				}
				tauSeen[e.Dst] = struct{}{}
				extend(e.Dst, i, append(path, e), tauSeen)
				delete(tauSeen, e.Dst)
			}
		}
	}
	extend(d.StartEdge.Dst, 0, nil, map[csdf.StateID]struct{}{d.StartEdge.Dst: {}})
	return paths, nil
}

// outgoing indexes the edges by source, each list in canonical order so that
// paths come out in a reproducible order.
func outgoing(d *csdf.Diagram) map[csdf.StateID][]csdf.Edge {
	out := make(map[csdf.StateID][]csdf.Edge)
	for _, e := range d.Edges {
		out[e.Src] = append(out[e.Src], e)
	}
	for s := range out {
		csdf.SortEdges(out[s])
	}
	return out
}

// tauClosure adds to states every state reachable from them over tau edges.
func tauClosure(states map[csdf.StateID]struct{}, out map[csdf.StateID][]csdf.Edge) map[csdf.StateID]struct{} {
	queue := make([]csdf.StateID, 0, len(states))
	for s := range states {
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, e := range out[s] {
			if e.Event != csdf.Tau {
				continue
			}
			if _, ok := states[e.Dst]; !ok {
				states[e.Dst] = struct{}{}
				queue = append(queue, e.Dst)
			}
		}
	}
	return states
}

func sortedStates(states map[csdf.StateID]struct{}) []csdf.StateID {
	ss := make([]csdf.StateID, 0, len(states))
	for s := range states {
		ss = append(ss, s)
	}
	slices.Sort(ss)
	return ss
}

func enabledEvents(states map[csdf.StateID]struct{}, out map[csdf.StateID][]csdf.Edge) []csdf.Event {
	seen := make(map[csdf.Event]struct{})
	for s := range states {
		for _, e := range out[s] {
			if e.Event != csdf.Tau {
				seen[e.Event] = struct{}{}
			}
		}
	}
	events := make([]csdf.Event, 0, len(seen))
	for e := range seen {
		events = append(events, e)
	}
	slices.Sort(events)
	return events
}
