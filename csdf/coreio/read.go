// Package coreio reads and writes csdf/core diagrams in the .puml syntax
// (docs/SYNTAX.md), their clauses in the csdf/clause syntax.
package coreio

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/v4/csdf"
	"github.com/Kuniwak/puml-parallel/v4/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v4/csdf/core"
	"github.com/Kuniwak/puml-parallel/v4/csdf/types"
)

// ReadAuthored reads a diagram an author wrote: its clauses are read by
// clause.ParseAuthored, and each ∃ binds a name apart from those of the
// diagram (core.Diagram.BindersApart).
func ReadAuthored(source string) (*core.Diagram, error) {
	d, err := read(source, clause.ParseAuthored)
	if err != nil {
		return nil, fmt.Errorf("coreio.ReadAuthored: %w", err)
	}
	if err := d.BindersApart(); err != nil {
		return nil, fmt.Errorf("coreio.ReadAuthored: %w", err)
	}
	return d, nil
}

// Read reads a diagram a tool wrote, such as Write's: its clauses are read
// by clause.Parse, so they may hold ∃.
func Read(source string) (*core.Diagram, error) {
	d, err := read(source, clause.Parse)
	if err != nil {
		return nil, fmt.Errorf("coreio.Read: %w", err)
	}
	return d, nil
}

// Lower is the diagram tree says, what the tools evaluate: the plain
// diagram csdf.Plain reads of it, its clauses read by clause.Parse, as Read
// reads them. A tree holding promotion directives is refused
// (csdf.PromotionHintError): a core.Diagram has no place for them.
func Lower(tree *csdf.SyntaxTree) (*core.Diagram, error) {
	d, err := lowerTree(tree, clause.Parse)
	if err != nil {
		return nil, fmt.Errorf("coreio.Lower: %w", err)
	}
	return d, nil
}

// read reads the syntax tree of source, csdf.Parse's, and lowers it, each
// clause by parse.
func read(source string, parse func(string) (clause.And, error)) (*core.Diagram, error) {
	tree, err := csdf.Parse(source)
	if err != nil {
		return nil, err
	}
	return lowerTree(tree, parse)
}

// lowerTree lowers the plain diagram tree says, each clause by parse.
func lowerTree(tree *csdf.SyntaxTree, parse func(string) (clause.And, error)) (*core.Diagram, error) {
	src, err := csdf.Plain(tree)
	if err != nil {
		return nil, err
	}
	return lower(src, parse)
}

// lower is the diagram src says, each clause read by parse: what the tools
// evaluate.
func lower(src *csdf.Diagram, parse func(string) (clause.And, error)) (*core.Diagram, error) {
	check, clauses := parseOnce(parse)
	if err := checkClauses(src, check); err != nil {
		return nil, err
	}
	d := &core.Diagram{
		Name:     src.Name,
		Vars:     map[string][]string{},
		Init:     string(src.StartEdge.Dst),
		InitPost: clauses(src.StartEdge.Post),
		InitLine: src.StartEdge.Line,
	}
	for _, s := range csdf.SortedStates(src.States) {
		d.States = append(d.States, string(s.ID))
		if s.State.Name != string(s.ID) {
			if d.Labels == nil {
				d.Labels = map[string]string{}
			}
			d.Labels[string(s.ID)] = s.State.Name
		}
		for _, v := range s.State.Vars {
			d.Vars[string(s.ID)] = append(d.Vars[string(s.ID)], string(v.Name))
			if v.Type == "" {
				continue
			}
			t, err := types.Parse(v.Type)
			if err != nil {
				return nil, fmt.Errorf("line %d: the variable %s of %s: %w", s.State.Line, v.Name, s.ID, err)
			}
			if d.Types == nil {
				d.Types = map[string]types.Type{}
			}
			if err := core.DeclareType(d.Types, string(v.Name), t); err != nil {
				return nil, fmt.Errorf("line %d: %w", s.State.Line, err)
			}
		}
	}
	for _, c := range src.Channels {
		if _, ok := d.Channels[c.Name]; ok {
			return nil, fmt.Errorf("line %d: the channel %s is declared twice", c.Line, c.Name)
		}
		if d.Channels == nil {
			d.Channels = map[string][]types.Type{}
		}
		d.Channels[c.Name] = c.Fields
	}
	for _, e := range src.Edges {
		edge := edgeOf(e, clauses)
		for _, f := range edge.Event.Received() {
			if core.ReceivesOver(f, d.Vars[edge.Src], d.Vars[edge.Dst]) {
				return nil, fmt.Errorf("line %d: the edge %s -> %s receives %s, which names a state variable of %s or %s, over which it would bind", e.Line, edge.Src, edge.Dst, f, edge.Src, edge.Dst)
			}
		}
		d.Edges = append(d.Edges, edge)
	}
	for _, a := range src.Assumptions {
		d.Assumes = append(d.Assumes, core.Assumption{ID: a.ID, Text: string(a.Text)})
	}
	if e := src.EndEdge; e != nil {
		d.Ends = append(d.Ends, core.End{Src: string(e.Src), Guard: clauses(e.Guard)})
	}
	d.Observes = observationOf(src.Observes)
	if err := d.NamesBound(); err != nil {
		return nil, err
	}
	if err := core.TypesDeclared(d); err != nil {
		return nil, err
	}
	return d, nil
}

// readEdge reads one edge line, written at line, its clauses by parse.
func readEdge(text string, line int, parse func(string) (clause.And, error)) (core.Edge, error) {
	check, clauses := parseOnce(parse)
	e, err := csdf.ParseEdgeLine(text, line)
	if err != nil {
		return core.Edge{}, err
	}
	if err := checkClause(e.Line, "guard", e.Guard, check); err != nil {
		return core.Edge{}, err
	}
	if err := checkClause(e.Line, "post", e.Post, check); err != nil {
		return core.Edge{}, err
	}
	return edgeOf(e, clauses), nil
}

// writtenClause is a clause of a diagram: the line it is on and which of
// the line it is.
type writtenClause struct {
	line  int
	which string
	pred  csdf.Predicate
}

// checkClauses reads every clause of src by check, in the order of the
// lines they are on.
func checkClauses(src *csdf.Diagram, check func(csdf.Predicate) error) error {
	cs := []writtenClause{{src.StartEdge.Line, "post", src.StartEdge.Post}}
	for _, e := range src.Edges {
		cs = append(cs, writtenClause{e.Line, "guard", e.Guard}, writtenClause{e.Line, "post", e.Post})
	}
	if e := src.EndEdge; e != nil {
		cs = append(cs, writtenClause{e.Line, "guard", e.Guard})
	}
	slices.SortStableFunc(cs, func(a, b writtenClause) int { return a.line - b.line })
	for _, c := range cs {
		if err := checkClause(c.line, c.which, c.pred, check); err != nil {
			return err
		}
	}
	return nil
}

// checkClause refuses a predicate check refuses, saying the line it was
// written on and which predicate of the line it is.
func checkClause(line int, which string, pred csdf.Predicate, check func(csdf.Predicate) error) error {
	if err := check(pred); err != nil {
		return csdf.NewParseError(line, 0, fmt.Errorf("line %d, the %s: %w", line, which, err))
	}
	return nil
}

// parseOnce gives a check that reads each predicate by parse, keeping what
// it read, and clauses that give back what check kept, so no predicate is
// read twice. Edges whose clauses read the same share what was read, so
// nothing may change a clause in place.
func parseOnce(parse func(string) (clause.And, error)) (func(csdf.Predicate) error, func(csdf.Predicate) clause.And) {
	read := map[csdf.Predicate]clause.And{}
	check := func(p csdf.Predicate) error {
		if _, ok := read[p]; ok {
			return nil
		}
		f, err := parse(string(p))
		if err != nil {
			return err
		}
		read[p] = f
		return nil
	}
	clauses := func(p csdf.Predicate) clause.And { return read[p] }
	return check, clauses
}

func edgeOf(e csdf.Edge, clauses func(csdf.Predicate) clause.And) core.Edge {
	return core.Edge{
		Src:   string(e.Src),
		Dst:   string(e.Dst),
		Event: eventOf(e.Event),
		Guard: clauses(e.Guard),
		Post:  clauses(e.Post),
		Line:  e.Line,
	}
}

func eventOf(e csdf.Event) core.Event {
	out := core.Event{Channel: e.Channel()}
	for _, f := range e.Fields() {
		out.Fields = append(out.Fields, core.Field{Send: f.Dir == csdf.Sent, Name: f.Name})
	}
	return out
}

// observationOf is the observation the CSDF-OBSERVE lines os declare, nil
// when there is none.
func observationOf(os []csdf.Observe) *core.Observation {
	if len(os) == 0 {
		return nil
	}
	out := &core.Observation{}
	for _, o := range os {
		d := core.Observable{Name: o.Name}
		if o.IsView() {
			d.States = []string{}
			for _, s := range o.States {
				d.States = append(d.States, string(s))
			}
		}
		out.Observables = append(out.Observables, d)
	}
	return out
}
