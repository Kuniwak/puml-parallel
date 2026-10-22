package tracechk

import (
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// Match decides whether an event of the trace is a given event of the
// diagram, its label (Label). Events are free-form text and their notation is
// not fixed, so the rules shipped here are deliberately simple; anything
// richer is for sed or qhs to do to the trace before it gets here, or for a
// caller to supply.
type Match func(diagramEvent, traceEvent string) bool

// MatchExact accepts an event of the trace only as the identical text.
func MatchExact(diagramEvent, traceEvent string) bool {
	return diagramEvent == traceEvent
}

// MatchPrefix accepts an event of the trace as every diagram event it is a
// prefix of, so a trace stripped of its fields ("insert") matches the label
// with them ("insert?coin"). It includes the identical text.
func MatchPrefix(diagramEvent, traceEvent string) bool {
	return strings.HasPrefix(diagramEvent, traceEvent)
}

// Label is the event e as a diagram writes it: its channel, then ?name for
// each field it receives and !name for each it sends, in order.
func Label(e core.Event) string {
	var sb strings.Builder
	sb.WriteString(e.Channel)
	for _, f := range e.Fields {
		if f.Send {
			sb.WriteString("!")
		} else {
			sb.WriteString("?")
		}
		sb.WriteString(f.Name)
	}
	return sb.String()
}

// Trace is one event sequence to check, named after where it came from.
type Trace struct {
	Name   string
	Events []string
}

// Path is one way the diagram performs a prefix of the trace: the edges it
// takes in order, tau edges included, from the start state.
type Path []core.Edge

// PrefixPath is a Path together with what the diagram can do where it ends.
// Dst is stable exactly when none of Taus is enabled, and it can perform the
// next event of the trace exactly when some edge of Next is enabled.
type PrefixPath struct {
	Path Path
	Dst  string
	Taus []core.Edge
	Next []core.Edge
}

// Step is one event of the trace and every way the diagram can have performed
// the prefix before it. The trace is a trace of the diagram only if, along every
// path whose clauses hold, the state reached either is unstable or can
// perform Event.
type Step struct {
	Index int
	Event string
	Paths []PrefixPath
}

// Rejection says where the trace stops being one, even with every guard true:
// after Index events, the diagram may be in a stable state that has no edge for
// Event, so it may refuse Event there.
type Rejection struct {
	Index int
	Event string
	// States are the states the diagram may be in before Event, tau-closed
	// and sorted.
	States []string
	// Refusing are the stable ones among them with no edge for Event, sorted.
	// It is empty only when nothing can perform Event and no state is stable,
	// i.e. the diagram diverges instead.
	Refusing []string
	// Enabled are the visible events the refusing states can perform, sorted.
	Enabled []string
	// Paths are the ways the diagram reaches a refusing state. The refusal is
	// real unless the clauses along every one of them are unsatisfiable.
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
func CheckAll(m Match, d *core.Diagram, traces []Trace) []Result {
	results := make([]Result, 0, len(traces))
	for _, trace := range traces {
		results = append(results, Check(m, d, trace))
	}
	return results
}

// AnyRejected reports whether some result is a rejection.
func AnyRejected(results []Result) bool {
	return slices.ContainsFunc(results, func(r Result) bool { return !r.Accepted() })
}

// graph is the edges of a diagram by source, in the order the diagram lists
// them.
type graph map[string][]core.Edge

func graphOf(d *core.Diagram) graph {
	g := graph{}
	for _, e := range d.Edges {
		g[e.Src] = append(g[e.Src], e)
	}
	return g
}

func (g graph) taus(s string) []core.Edge {
	var ts []core.Edge
	for _, e := range g[s] {
		if e.Event.Channel == core.Tau {
			ts = append(ts, e)
		}
	}
	return ts
}

func (g graph) next(m Match, s, event string) []core.Edge {
	var ns []core.Edge
	for _, e := range g[s] {
		if e.Event.Channel != core.Tau && m(Label(e.Event), event) {
			ns = append(ns, e)
		}
	}
	return ns
}

func (g graph) tauClosure(states map[string]bool) map[string]bool {
	out := map[string]bool{}
	var todo []string
	for s := range states {
		out[s] = true
		todo = append(todo, s)
	}
	for len(todo) > 0 {
		s := todo[0]
		todo = todo[1:]
		for _, e := range g.taus(s) {
			if !out[e.Dst] {
				out[e.Dst] = true
				todo = append(todo, e.Dst)
			}
		}
	}
	return out
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
// The clauses are not evaluated here; the result carries them so that the
// caller can state the obligation.
func Check(m Match, d *core.Diagram, t Trace) Result {
	g := graphOf(d)
	trace := t.Events

	states := g.tauClosure(map[string]bool{d.Init: true})
	for i, event := range trace {
		next := map[string]bool{}
		refusing := map[string]bool{}
		for s := range states {
			ns := g.next(m, s, event)
			for _, e := range ns {
				next[e.Dst] = true
			}
			if len(ns) == 0 && len(g.taus(s)) == 0 {
				refusing[s] = true
			}
		}
		if len(refusing) > 0 || len(next) == 0 {
			paths := prefixPaths(m, d, g, trace, i+1)[i]
			if len(refusing) > 0 {
				paths = slices.DeleteFunc(paths, func(p PrefixPath) bool { return !refusing[p.Dst] })
			}
			return Result{Trace: t, Rejection: &Rejection{
				Index:    i,
				Event:    event,
				States:   sorted(states),
				Refusing: sorted(refusing),
				Enabled:  enabledEvents(refusing, g),
				Paths:    paths,
			}}
		}
		states = g.tauClosure(next)
	}

	byIndex := prefixPaths(m, d, g, trace, len(trace))
	steps := make([]Step, 0, len(trace))
	for i, event := range trace {
		steps = append(steps, Step{Index: i, Event: event, Paths: byIndex[i]})
	}
	return Result{Trace: t, Steps: steps}
}

// prefixPaths enumerates, for every i < upto, the paths performing the first i
// events of the trace, in a deterministic depth-first order.
func prefixPaths(m Match, d *core.Diagram, g graph, trace []string, upto int) [][]PrefixPath {
	byIndex := make([][]PrefixPath, upto)
	var extend func(s string, i int, path Path, tauSeen map[string]bool)
	extend = func(s string, i int, path Path, tauSeen map[string]bool) {
		if i >= upto {
			return
		}
		byIndex[i] = append(byIndex[i], PrefixPath{
			Path: slices.Clone(path),
			Dst:  s,
			Taus: g.taus(s),
			Next: g.next(m, s, trace[i]),
		})
		for _, e := range g[s] {
			switch {
			case e.Event.Channel == core.Tau:
				if tauSeen[e.Dst] {
					continue
				}
				tauSeen[e.Dst] = true
				extend(e.Dst, i, append(path, e), tauSeen)
				delete(tauSeen, e.Dst)
			case m(Label(e.Event), trace[i]):
				extend(e.Dst, i+1, append(path, e), map[string]bool{e.Dst: true})
			}
		}
	}
	extend(d.Init, 0, nil, map[string]bool{d.Init: true})
	return byIndex
}

func sorted(states map[string]bool) []string {
	ss := make([]string, 0, len(states))
	for s := range states {
		ss = append(ss, s)
	}
	slices.Sort(ss)
	return ss
}

func enabledEvents(states map[string]bool, g graph) []string {
	seen := map[string]bool{}
	for s := range states {
		for _, e := range g[s] {
			if e.Event.Channel != core.Tau {
				seen[Label(e.Event)] = true
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	return sorted(seen)
}
