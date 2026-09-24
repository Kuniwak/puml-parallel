// Package transtable tabulates a Composable State Diagram as a state transition
// table: a row per state, a column per event, and in every cell what may happen
// when the environment offers that event in that state - where the diagram may
// go, and when it may refuse.
package transtable

import (
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Table is the state transition table of a diagram.
type Table struct {
	Columns []Column
	Rows    []Row
}

// Column is one event the environment may offer.
type Column struct {
	Event csdf.Event
}

// Row is one state of the diagram. Cells holds, for every column in order,
// the outcomes of offering that column's event in this state.
type Row struct {
	State csdf.StateID
	Name  string
	Cells [][]Outcome
}

// Outcome is one thing that may happen when an event is offered: the diagram
// accepts it and goes to Dst, or it refuses it. It may happen when Cond holds.
type Outcome struct {
	Cond Cond
	// Posts are the postconditions along the way, in order.
	Posts   []csdf.Predicate
	Refused bool
	Dst     csdf.StateID
}

// Build tabulates d. It fails with a *LivelockError when a tau cycle is
// reachable, since the table reads d in the stable-failures sense and that
// sense is blind to divergence.
func Build(d *csdf.Diagram) (*Table, error) {
	if livelock, ok := csdf.CheckLivelockFree(d); !ok {
		return nil, &LivelockError{Livelock: livelock}
	}

	x := index{out: outgoing(d)}
	states := reachable(d.StartEdge.Dst, x.out)
	columns := columnsOf(states, x)

	rows := make([]Row, 0, len(states))
	for _, s := range states {
		cells := make([][]Outcome, len(columns))
		for i, c := range columns {
			cells[i] = x.outcomes(s, c)
		}
		rows = append(rows, Row{State: s, Name: d.States[s].Name, Cells: cells})
	}
	return &Table{Columns: columns, Rows: rows}, nil
}

// index is the diagram arranged for looking up what a state may do.
type index struct {
	out map[csdf.StateID][]csdf.Edge
}

// take is one way a state may perform a column: under its guard, adding its
// postconditions, to dst.
type take struct {
	guard csdf.Predicate
	posts []csdf.Predicate
	dst   csdf.StateID
}

// takes lists the ways u may perform c, in canonical order.
func (x index) takes(u csdf.StateID, c Column) []take {
	var ts []take
	for _, e := range x.out[u] {
		if e.Event == c.Event {
			ts = append(ts, take{guard: e.Guard, posts: []csdf.Predicate{e.Post}, dst: e.Dst})
		}
	}
	return ts
}

// outcomes lists what may happen when c is offered in state s, in the
// stable-failures sense. The diagram may first take any number of tau edges
// whose guards hold, since the environment cannot see them; then, in the state
// u it has reached, it may perform c in a way whose guard holds, or, when u is
// stable (no tau guard holds) and no guard for c holds, refuse c. Each outcome
// is conditioned on every guard along its path, in order, and the outcomes come
// in the order of a depth-first walk.
//
// The walk terminates only when no tau cycle is reachable from s.
func (x index) outcomes(s csdf.StateID, c Column) []Outcome {
	var os []Outcome
	var visit func(u csdf.StateID, cond Cond, posts []csdf.Predicate)
	visit = func(u csdf.StateID, cond Cond, posts []csdf.Predicate) {
		var takeGuards, tauGuards []csdf.Predicate
		for _, t := range x.takes(u, c) {
			os = append(os, Outcome{Cond: cond.and(t.guard), Posts: slices.Concat(posts, t.posts), Dst: t.dst})
			takeGuards = append(takeGuards, t.guard)
		}
		taus := tauEdges(x.out[u])
		for _, e := range taus {
			tauGuards = append(tauGuards, e.Guard)
		}
		// u refuses c when it is stable and cannot perform c, which is
		// impossible as soon as one of those guards is true.
		if !slices.ContainsFunc(takeGuards, csdf.IsTrue) && !slices.ContainsFunc(tauGuards, csdf.IsTrue) {
			os = append(os, Outcome{Cond: cond.andNot(tauGuards).andNot(takeGuards), Posts: posts, Refused: true})
		}
		for _, e := range taus {
			visit(e.Dst, cond.and(e.Guard), append(slices.Clip(posts), e.Post))
		}
	}
	visit(s, nil, nil)
	return os
}

func tauEdges(edges []csdf.Edge) []csdf.Edge {
	var taus []csdf.Edge
	for _, e := range edges {
		if e.Event == csdf.Tau {
			taus = append(taus, e)
		}
	}
	return taus
}

// reachable returns the states reachable from start, in the order a
// breadth-first walk meets them when it follows edges in canonical order.
func reachable(start csdf.StateID, out map[csdf.StateID][]csdf.Edge) []csdf.StateID {
	order := []csdf.StateID{start}
	seen := map[csdf.StateID]struct{}{start: {}}
	for i := 0; i < len(order); i++ {
		for _, e := range out[order[i]] {
			if _, ok := seen[e.Dst]; !ok {
				seen[e.Dst] = struct{}{}
				order = append(order, e.Dst)
			}
		}
	}
	return order
}

// columnsOf returns the visible events of states, in the order the states and
// their edges come.
func columnsOf(states []csdf.StateID, x index) []Column {
	var columns []Column
	// The environment cannot offer tau, so it is not a column.
	seen := map[csdf.Event]struct{}{csdf.Tau: {}}
	for _, s := range states {
		for _, e := range x.out[s] {
			if _, ok := seen[e.Event]; !ok {
				seen[e.Event] = struct{}{}
				columns = append(columns, Column{Event: e.Event})
			}
		}
	}
	return columns
}

// outgoing indexes the edges by source, each list in canonical order so that
// the table comes out the same for the same diagram.
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
