// Package transtable tabulates a Composable State Diagram as a state transition
// table: a row per state, a column per event, and in every cell what may happen
// when the environment offers that event in that state - where the diagram may
// go, and when it may refuse.
package transtable

import (
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

// Build tabulates d.
func Build(d *csdf.Diagram) (*Table, error) {
	out := outgoing(d)
	states := reachable(d.StartEdge.Dst, out)
	columns := columnsOf(states, out)

	rows := make([]Row, 0, len(states))
	for _, s := range states {
		cells := make([][]Outcome, len(columns))
		for i, c := range columns {
			cells[i] = outcomes(out[s], c.Event)
		}
		rows = append(rows, Row{State: s, Name: d.States[s].Name, Cells: cells})
	}
	return &Table{Columns: columns, Rows: rows}, nil
}

// outcomes lists what may happen when event is offered to a state with the
// given edges: it takes an edge for the event when that edge's guard holds, and
// it refuses the event when no such guard holds.
func outcomes(edges []csdf.Edge, event csdf.Event) []Outcome {
	var os []Outcome
	var guards []csdf.Predicate
	unconditional := false
	for _, e := range edges {
		if e.Event != event {
			continue
		}
		os = append(os, Outcome{Cond: Cond(nil).and(e.Guard), Posts: []csdf.Predicate{e.Post}, Dst: e.Dst})
		unconditional = unconditional || csdf.IsTrue(e.Guard)
		guards = append(guards, e.Guard)
	}
	if unconditional {
		return os
	}
	return append(os, Outcome{Cond: Cond(nil).andNot(guards), Refused: true})
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
func columnsOf(states []csdf.StateID, out map[csdf.StateID][]csdf.Edge) []Column {
	var columns []Column
	seen := make(map[csdf.Event]struct{})
	for _, s := range states {
		for _, e := range out[s] {
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
