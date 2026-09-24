// Package transtable tabulates a Composable State Diagram as a state transition
// table: a row per state, a column per event, and in every cell what may happen
// when the environment offers that event in that state - where the diagram may
// go, and when it may refuse.
package transtable

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Table is the state transition table of a diagram.
type Table struct {
	Columns []Column
	Rows    []Row
	// Unreachable are the states the diagram can never be in, sorted. They
	// take no part in its behaviour, so they have no row and their events no
	// column; their IDs are kept so that a missing row is never a surprise.
	Unreachable []csdf.StateID
}

// Column is one event the environment may offer, or termination.
type Column struct {
	Event csdf.Event
	// Termination marks the column of successful termination, which an end
	// edge performs. Event is empty then.
	Termination bool
}

// Terminated is where an outcome of the termination column leads. It can never
// be the ID of a state, since an ID has no brackets.
const Terminated csdf.StateID = "[*]"

// Row is one state of the diagram. Cells holds, for every column in order,
// the outcomes of offering that column's event in this state.
type Row struct {
	State csdf.StateID
	Name  string
	Cells [][]Outcome
}

// Outcome is one thing that may happen when an event is offered: the diagram
// accepts it and goes to Dst, or it refuses it. It may happen when Cond holds
// of the values x of the row's state and the parameters c of the offered
// event, for some values x' after an accepted event. Cond is simplified, and
// logic.True when the outcome is unconditional.
type Outcome struct {
	Cond    logic.Formula
	Refused bool
	Dst     csdf.StateID
}

// Build tabulates d. It fails with a *LivelockError when a tau cycle is
// reachable, since the table reads d in the stable-failures sense and that
// sense is blind to divergence.
//
// Every postcondition is taken to admit some values after its step, whatever
// the parameters and the values before. That premise is what lets an edge be
// enabled exactly when its guard holds, which is where refusals come from.
func Build(d *csdf.Diagram) (*Table, error) {
	if livelock, ok := csdf.CheckLivelockFree(d); !ok {
		return nil, &LivelockError{Livelock: livelock}
	}

	// Each list is in canonical order, so the table comes out the same for the
	// same diagram.
	x := index{out: csdf.Outgoing(d), end: d.EndEdge}
	states := csdf.Reachable(d.StartEdge.Dst, x.out)
	columns := columnsOf(states, x)

	rows := make([]Row, 0, len(states))
	for _, s := range states {
		cells := make([][]Outcome, len(columns))
		for i, c := range columns {
			cells[i] = x.outcomes(s, c)
		}
		rows = append(rows, Row{State: s, Name: d.States[s].Name, Cells: cells})
	}
	return &Table{Columns: columns, Rows: rows, Unreachable: unreachable(d, states)}, nil
}

func unreachable(d *csdf.Diagram, reachable []csdf.StateID) []csdf.StateID {
	seen := make(map[csdf.StateID]struct{}, len(reachable))
	for _, id := range reachable {
		seen[id] = struct{}{}
	}
	var ids []csdf.StateID
	for id := range d.States {
		if _, ok := seen[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// index is the diagram arranged for looking up what a state may do.
type index struct {
	out map[csdf.StateID][]csdf.Edge
	end *csdf.EndEdge
}

// take is one way a state may perform a column: its guard over the values v,
// what holds when it is taken from v, and where it leads.
type take struct {
	guard func(v logic.Var) logic.Formula
	taken func(v logic.Var) logic.Formula
	dst   csdf.StateID
}

// takes lists the ways u may perform c, in canonical order. An edge for an
// event reads the parameters c the environment offers; an end edge has no
// event and reads the values only, and it has no postcondition.
func (x index) takes(u csdf.StateID, c Column) []take {
	if c.Termination {
		if x.end == nil || x.end.Src != u {
			return nil
		}
		g := x.end.Guard
		guard := func(v logic.Var) logic.Formula { return pred(g, v) }
		return []take{{guard: guard, taken: guard, dst: Terminated}}
	}
	var ts []take
	for _, e := range x.out[u] {
		if e.Event != c.Event {
			continue
		}
		g, p := e.Guard, e.Post
		guard := func(v logic.Var) logic.Formula { return pred(g, OfferedParams, v) }
		ts = append(ts, take{
			guard: guard,
			taken: func(v logic.Var) logic.Formula {
				return logic.And(guard(v), pred(p, OfferedParams, v, NextValues))
			},
			dst: e.Dst,
		})
	}
	return ts
}

// tauStep is a tau edge a walk took.
type tauStep struct {
	guard, post csdf.Predicate
}

// conjuncts are what the tau steps conjoin: the guard of the i-th step applied
// to its parameters and the values before it, and its postcondition to those
// and the values after it.
func conjuncts(steps []tauStep) []logic.Formula {
	fs := make([]logic.Formula, 0, 2*len(steps))
	for i, s := range steps {
		n := i + 1
		fs = append(fs,
			pred(s.guard, ParamsOfStep(n), ValuesAfterStep(i)),
			pred(s.post, ParamsOfStep(n), ValuesAfterStep(i), ValuesAfterStep(n)),
		)
	}
	return fs
}

// closeOver conjoins what k tau steps collected and what holds after them,
// binds the variables of the steps, and simplifies.
func closeOver(k int, path []logic.Formula, then ...logic.Formula) logic.Formula {
	return logic.Simplify(logic.Exists(stepVars(k), logic.And(slices.Concat(path, then)...)))
}

// outcomes lists what may happen when c is offered in state s, in the
// stable-failures sense. The diagram may first take any number of tau edges,
// since the environment cannot see them; then, in the state u it has reached,
// it may perform c, or, when u is stable and cannot perform c, refuse it. The
// condition of an outcome conjoins, in path order, every guard and
// postcondition along its path, applied to the values each reads, and binds
// the values and hidden parameters in between. The outcomes come in the order
// of a depth-first walk.
//
// u is stable when none of its tau edges is enabled, and it cannot perform c
// when none of its ways of performing c is. Every postcondition admits some
// values after it, so an edge is enabled exactly when its guard holds for some
// parameters: those of a tau edge are hidden and so bound, those for c are the
// ones offered.
//
// The walk terminates only when no tau cycle is reachable from s. Two ways
// that reach a state along the same guards and postconditions lead to the same
// outcomes, so only the first is walked; otherwise the tau diamonds that
// hiding interleaved events makes would grow the walk exponentially.
func (x index) outcomes(s csdf.StateID, c Column) []Outcome {
	var os []Outcome
	walked := make(map[string]struct{})
	var visit func(u csdf.StateID, steps []tauStep)
	visit = func(u csdf.StateID, steps []tauStep) {
		key := walkKey(u, steps)
		if _, ok := walked[key]; ok {
			return
		}
		walked[key] = struct{}{}

		k := len(steps)
		here := ValuesAfterStep(k)
		path := conjuncts(steps)

		takes := x.takes(u, c)
		taus := csdf.TauEdges(x.out[u])
		for _, t := range takes {
			os = append(os, Outcome{Cond: closeOver(k, path, t.taken(here)), Dst: t.dst})
		}

		// u refuses c when it is stable and cannot perform c. A true guard
		// makes that false, and then there is no refusal to list.
		var refusal []logic.Formula
		for _, e := range taus {
			hidden := ParamsOfStep(k + 1)
			refusal = append(refusal, logic.Not(logic.Exists([]logic.Var{hidden}, pred(e.Guard, hidden, here))))
		}
		for _, t := range takes {
			refusal = append(refusal, logic.Not(t.guard(here)))
		}
		if cond := closeOver(k, path, refusal...); !logic.IsFalse(cond) {
			os = append(os, Outcome{Cond: cond, Refused: true})
		}

		for _, e := range taus {
			visit(e.Dst, append(slices.Clip(steps), tauStep{guard: e.Guard, post: e.Post}))
		}
	}
	visit(s, nil)
	return os
}

// walkKey identifies where a walk is and the tau steps it took. A predicate
// may hold any character but a semicolon, so no separator can be trusted;
// every field is written after its length instead, which makes the key tell
// apart any two walks that differ.
func walkKey(u csdf.StateID, steps []tauStep) string {
	var sb strings.Builder
	field := func(s string) { fmt.Fprintf(&sb, "%d:%s", len(s), s) }

	field(string(u))
	for _, s := range steps {
		field(string(s.guard))
		field(string(s.post))
	}
	return sb.String()
}

// columnsOf returns the visible events of states, in the order the states and
// their edges come, and then termination when one of the states may terminate.
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
	if x.end != nil && slices.Contains(states, x.end.Src) {
		columns = append(columns, Column{Termination: true})
	}
	return columns
}
