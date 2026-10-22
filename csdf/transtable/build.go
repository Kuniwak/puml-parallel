// Package transtable tabulates a Composable State Diagram as a state transition
// table: a row per state, a column per event, and in every cell what may happen
// when the environment offers that event in that state - where the diagram may
// go, and when it may refuse.
package transtable

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

// Table is the state transition table of a diagram.
type Table struct {
	Columns []Column
	Rows    []Row
	// Unreachable are the states the diagram can never be in, sorted. They
	// take no part in its behaviour, so they have no row and their events no
	// column; their IDs are kept so that a missing row is never a surprise.
	Unreachable []string
}

// Column is one column past the state and its name: an EventColumn or the
// TerminationColumn.
type Column interface {
	// Header is the column's header in a table.
	Header() string
	isColumn()
}

// EventColumn is the column of an event the environment may offer: of every
// edge of one shape (core.Event.SameShape), whatever it names its fields.
// Its header is the label of the first of them (core.Event.Label), and the
// fields are named as it names them.
type EventColumn struct{ Event core.Event }

// TerminationColumn is the column of successful termination, which an end
// edge performs. Its header is spelled the way PlantUML spells the end.
type TerminationColumn struct{}

func (c EventColumn) Header() string     { return c.Event.Label() }
func (TerminationColumn) Header() string { return "[*]" }
func (EventColumn) isColumn()            {}
func (TerminationColumn) isColumn()      {}

// Row is one state of the diagram. Cells holds, for every column of the table
// in the same order, the outcomes of offering that column's event in this
// state.
type Row struct {
	State string
	Name  string
	Cells [][]Outcome
}

// Outcome is one thing that may happen when an event is offered. It may happen
// when Cond holds of the values of the row's state and the fields the offered
// event receives, for some values after an accepted event (v'). Cond is
// simplified, and logic.True when the outcome is unconditional.
type Outcome struct {
	Cond   logic.Formula
	Result Result
}

// equal reports whether o and p are the same outcome: the same result under
// the same condition, structurally.
func (o Outcome) equal(p Outcome) bool {
	return o.Result == p.Result && logic.Equal(o.Cond, p.Cond)
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
type Goto struct{ State string }

// Terminate accepts termination: the diagram ends.
type Terminate struct{}

// Refuse refuses the event.
type Refuse struct{}

func (Goto) Accepted() bool      { return true }
func (Terminate) Accepted() bool { return true }
func (Refuse) Accepted() bool    { return false }
func (g Goto) String() string    { return "→ " + g.State }
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
// Every post is taken to admit some values after its step, whatever the
// fields and the values before. That premise is what lets an edge be enabled
// exactly when its guard holds, which is where refusals come from.
//
// conds says how much the conditions keep; one Build does not know, the zero
// Conditions among them, is refused.
func Build(d *core.Diagram, conds Conditions) (*Table, error) {
	if !conds.known() {
		return nil, fmt.Errorf("transtable.Build: %w", fmt.Errorf("unknown conditions %d; take them from ParseConditions", conds))
	}
	if ids := undeclared(d); ids != nil {
		return nil, &UndeclaredStateError{States: ids}
	}
	if states := d.TauCycleStates(); len(states) > 0 {
		return nil, &LivelockError{States: states}
	}

	ix := index{d: d, conds: conds}
	states := d.ReachableStates()
	columns := columnsOf(states, d)

	rows := make([]Row, 0, len(states))
	for _, s := range states {
		cells := make([][]Outcome, len(columns))
		for i, c := range columns {
			os, err := ix.outcomes(s, c)
			if err != nil {
				return nil, fmt.Errorf("transtable.Build: %w", err)
			}
			cells[i] = os
		}
		name := s
		if l, ok := d.Labels[s]; ok {
			name = l
		}
		rows = append(rows, Row{State: s, Name: name, Cells: cells})
	}
	return &Table{Columns: columns, Rows: rows, Unreachable: unreachable(d, states)}, nil
}

func unreachable(d *core.Diagram, reachable []string) []string {
	var ids []string
	for _, s := range d.States {
		if !slices.Contains(reachable, s) {
			ids = append(ids, s)
		}
	}
	slices.Sort(ids)
	return ids
}

// index is the diagram arranged for looking up what a state may do.
type index struct {
	d     *core.Diagram
	conds Conditions
}

func (ix index) out(s string) []int {
	var es []int
	for i, e := range ix.d.Edges {
		if e.Src == s {
			es = append(es, i)
		}
	}
	return es
}

func (ix index) taus(s string) []int {
	var ts []int
	for _, i := range ix.out(s) {
		if ix.d.Edges[i].Event.Channel == core.Tau {
			ts = append(ts, i)
		}
	}
	return ts
}

// take is one way a state may perform a column: its guard over the values
// after k tau steps, what holds when it is taken, and what it comes to.
type take struct {
	guard  func(k int) (logic.Formula, error)
	taken  func(k int) ([]conjunct, error)
	result Result
}

// conjunct is one of the formulas a condition conjoins. It is made by holds
// or by binding.
type conjunct struct {
	formula logic.Formula
	// binds are the values a post of a tau step leads to, which the
	// condition binds; empty for a conjunct that binds nothing, and for the
	// post of the accepted event, whose values after it stay free.
	binds []obligation.Value
	// post says the conjunct is a post, which ConditionsEnabling may leave
	// out when nothing kept reads what it binds.
	post bool
}

// holds is a conjunct that binds nothing: a guard, or a refusal's negation.
func holds(f logic.Formula) conjunct { return conjunct{formula: f} }

// takes lists the ways u may perform c, in the order of the diagram. An edge
// for an event reads the fields the environment offers; an end edge has no
// event and reads the values only, and it has no post.
func (ix index) takes(u string, c Column) []take {
	ec, ok := c.(EventColumn)
	if !ok {
		var ts []take
		for _, end := range ix.d.Ends {
			if end.Src != u {
				continue
			}
			guard := func(k int) (logic.Formula, error) {
				f, err := clauseFormula(end.Guard)
				if err != nil {
					return nil, err
				}
				return logic.Rename(f, ix.before(u, k)), nil
			}
			taken := func(k int) ([]conjunct, error) {
				g, err := guard(k)
				return []conjunct{holds(g)}, err
			}
			ts = append(ts, take{guard: guard, taken: taken, result: Terminate{}})
		}
		return ts
	}
	col := ec.Event
	var ts []take
	for _, i := range ix.out(u) {
		e := ix.d.Edges[i]
		if e.Event.Channel == core.Tau || !e.Event.SameShape(col) {
			continue
		}
		// What the edge's clauses read of its event, named as the column
		// names it, before the values of the steps before.
		m, sends := asColumn(e, col)
		named := func(f logic.Formula, k int) logic.Formula {
			mk := maps.Clone(m)
			for v, w := range ix.before(u, k) {
				mk[v] = w
			}
			return logic.Rename(f, mk)
		}
		// The guard holds where the edge is taken: the clause, and that the
		// column's values at its sent fields are those it sends.
		guard := func(k int) (logic.Formula, error) {
			g, err := clauseFormula(e.Guard)
			if err != nil {
				return nil, err
			}
			return named(logic.And(append([]logic.Formula{g}, sends...)...), k), nil
		}
		ts = append(ts, take{
			guard: guard,
			taken: func(k int) ([]conjunct, error) {
				g, err := guard(k)
				if err != nil {
					return nil, err
				}
				p, err := clauseFormula(e.Post)
				if err != nil {
					return nil, err
				}
				return []conjunct{holds(g), {formula: named(p, k), post: true}}, nil
			},
			result: Goto{State: e.Dst},
		})
	}
	return ts
}

// asColumn reads e as an event of the column col, which has e's shape: it
// maps each field e receives to the column's name at its position, and says,
// of each field e sends, that the column's value at its position is the one
// e sends. The column's names are fresh (columnsOf), so neither captures a
// name e reads.
func asColumn(e core.Edge, col core.Event) (map[logic.Var]logic.Var, []logic.Formula) {
	m := map[logic.Var]logic.Var{}
	var sends []logic.Formula
	for i, f := range e.Event.Fields {
		name := logic.Var(col.Fields[i].Name)
		if f.Send {
			sends = append(sends, logic.Eq(name, logic.Var(f.Name)))
		} else {
			m[logic.Var(f.Name)] = name
		}
	}
	return m, sends
}

// conjuncts are what the tau steps conjoin: the guard of the i-th step over
// the values before it, and its post over those and the values after it.
func (ix index) conjuncts(steps []int) ([]conjunct, error) {
	cs := make([]conjunct, 0, 2*len(steps))
	for i, s := range steps {
		e := ix.d.Edges[s]
		g, err := clauseFormula(e.Guard)
		if err != nil {
			return nil, err
		}
		p, err := clauseFormula(e.Post)
		if err != nil {
			return nil, err
		}
		m := ix.before(e.Src, i)
		after := ix.after(e.Dst, i+1)
		for k, v := range after {
			m[k] = v
		}
		binds := make([]obligation.Value, 0, len(after))
		for _, v := range ix.d.Vars[e.Dst] {
			binds = append(binds, obligation.After(v, i+1))
		}
		cs = append(cs, holds(logic.Rename(g, m)), conjunct{formula: logic.Rename(p, m), binds: binds, post: true})
	}
	return cs, nil
}

// closeOver conjoins what k tau steps collected and what holds after them,
// keeping what ix.conds says, binds the values of the steps, and simplifies.
func (ix index) closeOver(path []conjunct, then ...conjunct) logic.Formula {
	cs := ix.conds.discharge(slices.Concat(path, then))
	fs := make([]logic.Formula, len(cs))
	var bound []logic.Var
	var types []string
	for i, c := range cs {
		fs[i] = c.formula
		for _, b := range c.binds {
			bound, types = append(bound, b.Var), append(types, b.Type)
		}
	}
	return logic.Simplify(logic.ExistsTyped(bound, types, logic.And(fs...)))
}

// outcomes lists what may happen when c is offered in state s, in the
// stable-failures sense. The diagram may first take any number of tau edges,
// since the environment cannot see them; then, in the state u it has reached,
// it may perform c, or, when u is stable and cannot perform c, refuse it. The
// condition of an outcome conjoins, in path order, every guard and post along
// its path, over the values each reads, and binds the values in between. The
// outcomes come in the order of a depth-first walk, and an outcome that comes
// out the same as one before it, under logic.Equal, is listed once.
//
// u is stable when none of its tau edges is enabled, and it cannot perform c
// when none of its ways of performing c is. Every post admits some values
// after it, so an edge is enabled exactly when its guard holds: of the fields
// offered for c, and for a tau, which receives nothing, of the values alone.
//
// The walk terminates only when no tau cycle is reachable from s. Each way
// of tau steps is walked, so a diamond of them is walked once per way round;
// two ways reach a state along different clauses, so their outcomes differ.
func (ix index) outcomes(s string, c Column) ([]Outcome, error) {
	var os []Outcome
	add := func(o Outcome) {
		if !slices.ContainsFunc(os, o.equal) {
			os = append(os, o)
		}
	}
	var visit func(u string, steps []int) error
	visit = func(u string, steps []int) error {
		k := len(steps)
		path, err := ix.conjuncts(steps)
		if err != nil {
			return err
		}
		takes := ix.takes(u, c)
		for _, t := range takes {
			taken, err := t.taken(k)
			if err != nil {
				return err
			}
			add(Outcome{Cond: ix.closeOver(path, taken...), Result: t.result})
		}

		// u refuses c when it is stable and cannot perform c. A true guard
		// makes that false, and then there is no refusal to list.
		var refusal []conjunct
		for _, i := range ix.taus(u) {
			g, err := clauseFormula(ix.d.Edges[i].Guard)
			if err != nil {
				return err
			}
			refusal = append(refusal, holds(logic.Not(logic.Rename(g, ix.before(u, k)))))
		}
		for _, t := range takes {
			g, err := t.guard(k)
			if err != nil {
				return err
			}
			refusal = append(refusal, holds(logic.Not(g)))
		}
		if cond := ix.closeOver(path, refusal...); !logic.IsFalse(cond) {
			add(Outcome{Cond: cond, Result: Refuse{}})
		}

		for _, i := range ix.taus(u) {
			if err := visit(ix.d.Edges[i].Dst, append(slices.Clip(steps), i)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(s, nil); err != nil {
		return nil, err
	}
	return os, nil
}

// columnEvent is the event of the column of the edges of d of shape: its
// fields named after the first edge's, each made fresh against every name
// renaming an edge's field to it could capture - a state variable, the name
// a field of another position of an edge of the shape has, and the other
// fields of the column - so that every diagram has its columns.
func columnEvent(d *core.Diagram, shape core.Event, vars map[string]bool) core.Event {
	col := core.Event{Channel: shape.Channel}
	for i, f := range shape.Fields {
		taken := maps.Clone(vars)
		for _, e := range d.Edges {
			if !e.Event.SameShape(shape) {
				continue
			}
			for j, g := range e.Event.Fields {
				if j != i {
					taken[g.Name] = true
				}
			}
		}
		for _, g := range col.Fields {
			taken[g.Name] = true
		}
		col.Fields = append(col.Fields, core.Field{Send: f.Send, Name: clause.Fresh(taken, f.Name)})
	}
	return col
}

// columnsOf returns the visible events of states, in the order the states and
// their edges come, and then termination when one of the states may terminate.
func columnsOf(states []string, d *core.Diagram) []Column {
	vars := map[string]bool{}
	for _, vs := range d.Vars {
		for _, v := range vs {
			vars[v] = true
		}
	}
	var columns []Column
	var shapes []core.Event
	for _, s := range states {
		for _, e := range d.Edges {
			// The environment cannot offer tau, so it is not a column. Edges
			// of one shape are one event, so one column.
			if e.Src != s || e.Event.Channel == core.Tau || slices.ContainsFunc(shapes, e.Event.SameShape) {
				continue
			}
			shapes = append(shapes, e.Event)
			columns = append(columns, EventColumn{Event: columnEvent(d, e.Event, vars)})
		}
	}
	if slices.ContainsFunc(d.Ends, func(e core.End) bool { return slices.Contains(states, e.Src) }) {
		columns = append(columns, TerminationColumn{})
	}
	return columns
}
