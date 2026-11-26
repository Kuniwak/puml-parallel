// Package coreio reads and writes csdf/core diagrams in the .puml syntax
// (docs/SYNTAX.md), their clauses in the csdf/clause syntax.
package coreio

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v7/csdf"
	"github.com/Kuniwak/puml-parallel/v7/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v7/csdf/core"
	"github.com/Kuniwak/puml-parallel/v7/csdf/types"
)

// ReadAuthored reads a diagram an author wrote: its syntax tree
// (csdf.Parse), lowered by LowerAuthored.
func ReadAuthored(source string) (*core.Diagram, error) {
	tree, err := csdf.Parse("", source)
	if err != nil {
		return nil, fmt.Errorf("coreio.ReadAuthored: %w", err)
	}
	d, err := LowerAuthored(tree)
	if err != nil {
		return nil, fmt.Errorf("coreio.ReadAuthored: %w", err)
	}
	return d, nil
}

// Read reads a diagram a tool wrote, such as Write's: its syntax tree
// (csdf.Parse), lowered by Lower.
func Read(source string) (*core.Diagram, error) {
	tree, err := csdf.Parse("", source)
	if err != nil {
		return nil, fmt.Errorf("coreio.Read: %w", err)
	}
	d, err := Lower(tree)
	if err != nil {
		return nil, fmt.Errorf("coreio.Read: %w", err)
	}
	return d, nil
}

// Lower is the diagram tree says, what the tools evaluate, for a diagram a
// tool wrote: the plain diagram csdf.Plain reads of it, its clauses turned
// into formulas by clause.FormulaOf, so they may hold ∃ binding any name. A
// tree holding promotion directives is refused (csdf.PromotionHintError): a
// core.Diagram has no place for them.
func Lower(tree *csdf.SyntaxTree) (*core.Diagram, error) {
	d, err := lowerTree(tree, clause.FormulaOf)
	if err != nil {
		return nil, fmt.Errorf("coreio.Lower: %w", err)
	}
	return d, nil
}

// LowerAuthored is Lower for a diagram an author wrote: its clauses are
// turned into formulas by clause.AuthoredFormulaOf, and each ∃ binds a name
// apart from those of the diagram (core.Diagram.BindersApart).
func LowerAuthored(tree *csdf.SyntaxTree) (*core.Diagram, error) {
	d, err := lowerTree(tree, clause.AuthoredFormulaOf)
	if err != nil {
		return nil, fmt.Errorf("coreio.LowerAuthored: %w", err)
	}
	if err := d.BindersApart(); err != nil {
		return nil, fmt.Errorf("coreio.LowerAuthored: %w", err)
	}
	return d, nil
}

// formulaOf turns the syntax tree of a clause into its formula.
type formulaOf func(clause.Syntax) (clause.And, error)

// lowerTree is the diagram tree says, each clause turned into a formula by
// formula.
func lowerTree(tree *csdf.SyntaxTree, formula formulaOf) (*core.Diagram, error) {
	src, err := csdf.Plain(tree)
	if err != nil {
		return nil, err
	}
	initPost, err := formulaAt(src.StartEdge.Post, src.StartEdge.Line, "post", formula)
	if err != nil {
		return nil, err
	}
	d := &core.Diagram{
		Name:     src.Name,
		Vars:     map[string][]string{},
		Init:     string(src.StartEdge.Dst),
		InitPost: initPost,
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
		edge, err := edgeOf(e, formula)
		if err != nil {
			return nil, err
		}
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
		guard, err := formulaAt(e.Guard, e.Line, "guard", formula)
		if err != nil {
			return nil, err
		}
		d.Ends = append(d.Ends, core.End{Src: string(e.Src), Guard: guard})
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

// formulaAt is the formula of c, the clause written on line as the which of
// that line, true when c is not written. A refusal is tagged with the line.
func formulaAt(c *csdf.ClauseSyntax, line int, which string, formula formulaOf) (clause.And, error) {
	if c == nil {
		return clause.And{}, nil
	}
	f, err := formula(c.Syntax)
	if err != nil {
		return clause.And{}, csdf.NewParseError(line, 0, fmt.Errorf("line %d, the %s: %q: %w", line, which, c.Text, err))
	}
	return f, nil
}

// edgeOf lowers e, each clause turned into a formula by formula.
func edgeOf(e csdf.Edge, formula formulaOf) (core.Edge, error) {
	guard, err := formulaAt(e.Guard, e.Line, "guard", formula)
	if err != nil {
		return core.Edge{}, err
	}
	post, err := formulaAt(e.Post, e.Line, "post", formula)
	if err != nil {
		return core.Edge{}, err
	}
	return core.Edge{
		Src:   string(e.Src),
		Dst:   string(e.Dst),
		Event: eventOf(e.Event),
		Guard: guard,
		Post:  post,
		Line:  e.Line,
	}, nil
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
