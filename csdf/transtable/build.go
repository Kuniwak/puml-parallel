// Package transtable tabulates a Composable State Diagram as a state transition
// table: a row per state, a column per event, and in every cell what may happen
// when the environment offers that event in that state - where the diagram may
// go, and when it may refuse.
package transtable

import (
	"slices"

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

// Column is one column past the state and its name: an EventColumn or the
// TerminationColumn.
type Column interface {
	// Header is the column's header in a table.
	Header() string
	isColumn()
}

// EventColumn is the column of an event the environment may offer. Its
// header is the event's whole text, which is what the event is.
type EventColumn struct{ Event csdf.Event }

// TerminationColumn is the column of successful termination, which an end
// edge performs. Its header is spelled the way PlantUML spells the end.
type TerminationColumn struct{}

func (c EventColumn) Header() string     { return string(c.Event) }
func (TerminationColumn) Header() string { return "[*]" }
func (EventColumn) isColumn()            {}
func (TerminationColumn) isColumn()      {}

// Row is one state of the diagram. Cells holds, for every column of the table
// in the same order, the outcomes of offering that column's event in this
// state.
type Row struct {
	State csdf.StateID
	Name  string
	Cells [][]Outcome
}

// Outcome is one thing that may happen when an event is offered. It may happen
// when Cond holds of the values x of the row's state and the parameters c of
// the offered event, for some values x' after an accepted event. Cond is
// simplified, and logic.True when the outcome is unconditional.
type Outcome struct {
	Cond   logic.Formula
	Result Result
}

// Result is what an outcome comes to: Goto, Terminate or Refuse.
type Result interface {
	// Accepted reports whether the event is accepted.
	Accepted() bool
	// String spells the result as a table does.
	String() string
	isResult()
}

// Goto accepts the event and leaves the diagram in State.
type Goto struct{ State csdf.StateID }

// Terminate accepts termination: the diagram ends.
type Terminate struct{}

// Refuse refuses the event.
type Refuse struct{}

func (Goto) Accepted() bool      { return true }
func (Terminate) Accepted() bool { return true }
func (Refuse) Accepted() bool    { return false }
func (g Goto) String() string    { return "→ " + string(g.State) }
func (Terminate) String() string { return "→ [*]" }
func (Refuse) String() string    { return "×" }
func (Goto) isResult()           {}
func (Terminate) isResult()      {}
func (Refuse) isResult()         {}

// Build tabulates d. It fails with an *UndeclaredStateError when an edge names
// a state d does not declare, and with a *LivelockError when a tau cycle is
// reachable, since the table reads d in the stable-failures sense and that
// sense is blind to divergence.
//
// Every postcondition is taken to admit some values after its step, whatever
// the parameters and the values before. That premise is what lets an edge be
// enabled exactly when its guard holds, which is where refusals come from.
func Build(d *csdf.Diagram) (*Table, error) {
	if ids := undeclared(d); ids != nil {
		return nil, &UndeclaredStateError{States: ids}
	}
	if livelock, ok := csdf.CheckLivelockFree(d); !ok {
		return nil, &LivelockError{Livelock: livelock}
	}

	// Each list is in canonical order, so the table comes out the same for the
	// same diagram.
	x := index{graph: csdf.NewGraph(d), end: d.EndEdge}
	states := x.graph.Reachable(d.StartEdge.Dst)
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
	graph csdf.Graph
	end   *csdf.EndEdge
}

// take is one way a state may perform a column: its guard over the values v,
// what holds when it is taken from v, and what it comes to.
type take struct {
	guard  func(v logic.Var) logic.Formula
	taken  func(v logic.Var) logic.Formula
	result Result
}

// takes lists the ways u may perform c, in canonical order. An edge for an
// event reads the parameters c the environment offers; an end edge has no
// event and reads the values only, and it has no postcondition.
func (x index) takes(u csdf.StateID, c Column) []take {
	ec, ok := c.(EventColumn)
	if !ok {
		if x.end == nil || x.end.Src != u {
			return nil
		}
		g := x.end.Guard
		guard := func(v logic.Var) logic.Formula { return pred(g, v) }
		return []take{{guard: guard, taken: guard, result: Terminate{}}}
	}
	var ts []take
	for _, e := range x.graph.Out(u) {
		if e.Event != ec.Event {
			continue
		}
		g, p := e.Guard, e.Post
		guard := func(v logic.Var) logic.Formula { return pred(g, OfferedParams, v) }
		ts = append(ts, take{
			guard: guard,
			taken: func(v logic.Var) logic.Formula {
				return logic.And(guard(v), pred(p, OfferedParams, v, NextValues))
			},
			result: Goto{State: e.Dst},
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
// hiding interleaved events makes would grow the walk exponentially. The steps
// of a way are named by a path, which is the same for the same steps.
func (x index) outcomes(s csdf.StateID, c Column) []Outcome {
	var os []Outcome
	var paths pathNames
	walked := make(map[walk]bool)
	var visit func(u csdf.StateID, pathName int, steps []tauStep)
	visit = func(u csdf.StateID, pathName int, steps []tauStep) {
		here := walk{state: u, path: pathName}
		if walked[here] {
			return
		}
		walked[here] = true

		k := len(steps)
		values := ValuesAfterStep(k)
		path := conjuncts(steps)

		takes := x.takes(u, c)
		taus := x.graph.Taus(u)
		for _, t := range takes {
			os = append(os, Outcome{Cond: closeOver(k, path, t.taken(values)), Result: t.result})
		}

		// u refuses c when it is stable and cannot perform c. A true guard
		// makes that false, and then there is no refusal to list.
		var refusal []logic.Formula
		for _, e := range taus {
			hidden := ParamsOfStep(k + 1)
			refusal = append(refusal, logic.Not(logic.Exists([]logic.Var{hidden}, pred(e.Guard, hidden, values))))
		}
		for _, t := range takes {
			refusal = append(refusal, logic.Not(t.guard(values)))
		}
		if cond := closeOver(k, path, refusal...); !logic.IsFalse(cond) {
			os = append(os, Outcome{Cond: cond, Result: Refuse{}})
		}

		for _, e := range taus {
			step := tauStep{guard: e.Guard, post: e.Post}
			visit(e.Dst, paths.extend(pathName, step), append(slices.Clip(steps), step))
		}
	}
	visit(s, rootPath, nil)
	return os
}

// walk is where a walk is and the path of tau steps it took there.
type walk struct {
	state csdf.StateID
	path  int
}

// rootPath names the path of no steps.
const rootPath = 0

// pathNames names paths of tau steps: a path is its last step after the path
// before it, so two paths get the same name exactly when their steps are the
// same.
type pathNames map[struct {
	before int
	step   tauStep
}]int

// extend names the path of step after the path named before.
func (ns *pathNames) extend(before int, step tauStep) int {
	if *ns == nil {
		*ns = make(pathNames)
	}
	key := struct {
		before int
		step   tauStep
	}{before: before, step: step}
	if name, ok := (*ns)[key]; ok {
		return name
	}
	name := len(*ns) + 1
	(*ns)[key] = name
	return name
}

// columnsOf returns the visible events of states, in the order the states and
// their edges come, and then termination when one of the states may terminate.
func columnsOf(states []csdf.StateID, x index) []Column {
	var columns []Column
	// The environment cannot offer tau, so it is not a column.
	seen := map[csdf.Event]struct{}{csdf.Tau: {}}
	for _, s := range states {
		for _, e := range x.graph.Out(s) {
			if _, ok := seen[e.Event]; !ok {
				seen[e.Event] = struct{}{}
				columns = append(columns, EventColumn{Event: e.Event})
			}
		}
	}
	if x.end != nil && slices.Contains(states, x.end.Src) {
		columns = append(columns, TerminationColumn{})
	}
	return columns
}
