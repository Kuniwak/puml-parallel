package tracechk

import (
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Match decides whether an event of the trace is a given event of the diagram.
// Events are free-form text and their notation is not fixed, so the rules
// shipped here are deliberately simple; anything richer is for sed or qhs to do
// to the trace before it gets here, or for a caller to supply.
type Match func(diagramEvent, traceEvent csdf.Event) bool

// MatchExact accepts an event of the trace only as the identical text.
func MatchExact(diagramEvent, traceEvent csdf.Event) bool {
	return diagramEvent == traceEvent
}

// MatchPrefix accepts an event of the trace as every diagram event it is a
// prefix of, so a trace stripped of its parameters ("insert") matches the
// parameterised label ("insert(coin)"). It includes the identical text.
func MatchPrefix(diagramEvent, traceEvent csdf.Event) bool {
	return strings.HasPrefix(string(diagramEvent), string(traceEvent))
}

// Trace is one event sequence to check, named after where it came from.
type Trace struct {
	Name   string
	Events []csdf.Event
}

// Path is one way the diagram performs a prefix of the trace: the edges it
// takes in order, tau edges included, from the start state.
type Path []csdf.Edge

// PrefixPath is a Path together with what the diagram can do where it ends.
// Dst is stable exactly when none of Taus is enabled, and it can perform the
// next event of the trace exactly when some edge of Next is enabled.
type PrefixPath struct {
	Path Path
	Dst  csdf.StateID
	Taus []csdf.Edge
	Next []csdf.Edge
}

// Step is one event of the trace and every way the diagram can have performed
// the prefix before it. The trace is a trace of the diagram only if, along every
// path whose predicates hold, the state reached either is unstable or can
// perform Event.
type Step struct {
	Index int
	Event csdf.Event
	Paths []PrefixPath
}

// Rejection says where the trace stops being one, even with every guard true:
// after Index events, the diagram may be in a stable state that has no edge for
// Event, so it may refuse Event there.
type Rejection struct {
	Index int
	Event csdf.Event
	// States are the states the diagram may be in before Event, tau-closed
	// and sorted.
	States []csdf.StateID
	// Refusing are the stable ones among them with no edge for Event, sorted.
	// It is empty only when nothing can perform Event and no state is stable,
	// i.e. the diagram diverges instead.
	Refusing []csdf.StateID
	// Enabled are the visible events the refusing states can perform, sorted.
	Enabled []csdf.Event
	// Paths are the ways the diagram reaches a refusing state. The refusal is
	// real unless the predicates along every one of them are unsatisfiable.
	Paths []PrefixPath
}

// Result is the verdict on one trace: its steps when it is accepted, or where
// it was rejected.
type Result struct {
	Trace     Trace
	Steps     []Step
	Rejection *Rejection
}

// Accepted reports whether the trace passed the check with every guard true.
func (r Result) Accepted() bool { return r.Rejection == nil }

// CheckAll checks every trace against d under m, in order.
func CheckAll(m Match, d *csdf.Diagram, traces []Trace) []Result {
	results := make([]Result, 0, len(traces))
	for _, trace := range traces {
		results = append(results, Check(m, d, trace))
	}
	return results
}

// AnyRejected reports whether some result is a rejection.
func AnyRejected(results []Result) bool {
	for _, r := range results {
		if !r.Accepted() {
			return true
		}
	}
	return false
}

// Check reports whether t is a trace of d in the stable-failures sense, taking
// every guard as true: for every prefix, no stable state the diagram may reach
// by performing it refuses the next event. This is the reading under which an
// environment offering the events one at a time is guaranteed to have each
// accepted; under the plain traces reading a nondeterministic branch that gets
// stuck would not count.
//
// A path never repeats a state within one run of tau edges, so a tau cycle
// yields finitely many paths, and paths that would go round one are left out.
// Guards and postconditions are natural language and are not evaluated here;
// the result carries them so that the caller can state the obligation.
func Check(m Match, d *csdf.Diagram, t Trace) Result {
	out := outgoing(d)
	trace := t.Events

	states := tauClosure(map[csdf.StateID]struct{}{d.StartEdge.Dst: {}}, out)
	for i, event := range trace {
		next := make(map[csdf.StateID]struct{})
		refusing := make(map[csdf.StateID]struct{})
		for s := range states {
			canPerform := false
			for _, e := range out[s] {
				if e.Event != csdf.Tau && m(e.Event, event) {
					next[e.Dst] = struct{}{}
					canPerform = true
				}
			}
			if !canPerform && len(taus(out[s])) == 0 {
				refusing[s] = struct{}{}
			}
		}
		if len(refusing) > 0 || len(next) == 0 {
			paths := prefixPaths(m, d, out, trace, i+1)[i]
			if len(refusing) > 0 {
				paths = slices.DeleteFunc(paths, func(p PrefixPath) bool {
					_, ok := refusing[p.Dst]
					return !ok
				})
			}
			return Result{Trace: t, Rejection: &Rejection{
				Index:    i,
				Event:    event,
				States:   sortedStates(states),
				Refusing: sortedStates(refusing),
				Enabled:  enabledEvents(refusing, out),
				Paths:    paths,
			}}
		}
		states = tauClosure(next, out)
	}

	byIndex := prefixPaths(m, d, out, trace, len(trace))
	steps := make([]Step, 0, len(trace))
	for i, event := range trace {
		steps = append(steps, Step{Index: i, Event: event, Paths: byIndex[i]})
	}
	return Result{Trace: t, Steps: steps}
}

// prefixPaths enumerates, for every i < upto, the paths performing the first i
// events of the trace, in a deterministic depth-first order.
func prefixPaths(m Match, d *csdf.Diagram, out map[csdf.StateID][]csdf.Edge, trace []csdf.Event, upto int) [][]PrefixPath {
	byIndex := make([][]PrefixPath, upto)
	var extend func(s csdf.StateID, i int, path Path, tauSeen map[csdf.StateID]struct{})
	extend = func(s csdf.StateID, i int, path Path, tauSeen map[csdf.StateID]struct{}) {
		if i >= upto {
			return
		}
		byIndex[i] = append(byIndex[i], PrefixPath{
			Path: slices.Clone(path),
			Dst:  s,
			Taus: taus(out[s]),
			Next: nextEdges(m, out[s], trace[i]),
		})
		for _, e := range out[s] {
			switch {
			case e.Event == csdf.Tau:
				if _, seen := tauSeen[e.Dst]; seen {
					continue
				}
				tauSeen[e.Dst] = struct{}{}
				extend(e.Dst, i, append(path, e), tauSeen)
				delete(tauSeen, e.Dst)
			case m(e.Event, trace[i]):
				extend(e.Dst, i+1, append(path, e), map[csdf.StateID]struct{}{e.Dst: {}})
			}
		}
	}
	extend(d.StartEdge.Dst, 0, nil, map[csdf.StateID]struct{}{d.StartEdge.Dst: {}})
	return byIndex
}

func taus(edges []csdf.Edge) []csdf.Edge {
	var ts []csdf.Edge
	for _, e := range edges {
		if e.Event == csdf.Tau {
			ts = append(ts, e)
		}
	}
	return ts
}

func nextEdges(m Match, edges []csdf.Edge, event csdf.Event) []csdf.Edge {
	var ns []csdf.Edge
	for _, e := range edges {
		if e.Event != csdf.Tau && m(e.Event, event) {
			ns = append(ns, e)
		}
	}
	return ns
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
	if len(seen) == 0 {
		return nil
	}
	events := make([]csdf.Event, 0, len(seen))
	for e := range seen {
		events = append(events, e)
	}
	slices.Sort(events)
	return events
}
