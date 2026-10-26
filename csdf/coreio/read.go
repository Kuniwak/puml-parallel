// Package coreio reads and writes csdf/core diagrams in the .puml syntax
// (docs/SYNTAX.md), their clauses in the csdf/clause syntax.
package coreio

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// ReadAuthored reads a diagram an author wrote: its clauses are read by
// clause.ParseAuthored, so none holds ∃.
func ReadAuthored(source string) (*core.Diagram, error) {
	d, err := read(source, clause.ParseAuthored)
	if err != nil {
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

// read reads source with the structure csdf.Parser reads, each clause by
// parse.
func read(source string, parse func(string) (clause.And, error)) (*core.Diagram, error) {
	check := func(p csdf.Predicate) error {
		_, err := parse(string(p))
		return err
	}
	src, err := csdf.NewParserCheckingClauses(source, check).ParseAt()
	if err != nil {
		return nil, csdf.WithPromotionHint(err, source)
	}
	clauses := func(p csdf.Predicate) clause.And {
		f, _ := parse(string(p)) // check has read every predicate already
		return f
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
		}
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
	return d, nil
}

// readEdge reads one edge line, written at line, its clauses by parse.
func readEdge(text string, line int, parse func(string) (clause.And, error)) (core.Edge, error) {
	read := map[csdf.Predicate]clause.And{}
	check := func(p csdf.Predicate) error {
		f, err := parse(string(p))
		read[p] = f
		return err
	}
	e, err := csdf.ParseEdgeLine(text, line, check)
	if err != nil {
		return core.Edge{}, err
	}
	return edgeOf(e, func(p csdf.Predicate) clause.And { return read[p] }), nil
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
