// Package core is the constructors of docs/CLAUSE_DESIGN.md — composition,
// hiding and promotion — on diagrams whose clauses are in the csdf/clause
// representation, and the meaning they are tested against: a diagram
// expanded over finite types into an LTS (csdf/lts).
package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Tau is the channel of an internal event.
const Tau = "tau"

// Field is a field of an event: received (?Name) or sent (!Name).
type Field struct {
	Send bool
	Name string
}

// Event is a channel with its fields.
type Event struct {
	Channel string
	Fields  []Field
}

// SameShape says whether e and o are on one channel with fields of the
// same directions, in order, whatever their names.
func (e Event) SameShape(o Event) bool {
	if e.Channel != o.Channel || len(e.Fields) != len(o.Fields) {
		return false
	}
	for i := range e.Fields {
		if e.Fields[i].Send != o.Fields[i].Send {
			return false
		}
	}
	return true
}

// Edge is a transition: Src --> Dst : Event ; Guard ; Post.
type Edge struct {
	Src, Dst    string
	Event       Event
	Guard, Post clause.And
}

// End is an end edge: Src --> [*] : Guard.
type End struct {
	Src   string
	Guard clause.And
}

// Diagram is a diagram whose clauses are formulas. States are its states;
// Vars gives the state variables of those that hold any; Init is the state
// the start edge leads to and InitPost the post of that edge; Ends are its
// end edges, which only promotion reads, as deletions. Assumes are what its
// runs as a whole are assumed to satisfy, outside its meaning
// (docs/CLAUSE_DESIGN.md). Name and Labels are for the reader alone, outside
// its meaning too: the title of the diagram, and the label of each state
// labelled apart from its ID.
type Diagram struct {
	Name     string
	Labels   map[string]string
	States   []string
	Vars     map[string][]string
	Init     string
	InitPost clause.And
	Edges    []Edge
	Ends     []End
	Assumes  []Assumption
}

// Assumption is one assume note: ID names the note, Text is what it
// assumes, which no clause can say.
type Assumption struct {
	ID, Text string
}

// label is what s is labelled: its label, or its ID.
func (d *Diagram) label(s string) string {
	if l, ok := d.Labels[s]; ok {
		return l
	}
	return s
}

// declared reports, as an error, a state d names but does not declare.
func (d *Diagram) declared() error {
	has := func(s string) bool { return slices.Contains(d.States, s) }
	if !has(d.Init) {
		return fmt.Errorf("no state %s, which the start edge leads to", d.Init)
	}
	for s := range d.Vars {
		if !has(s) {
			return fmt.Errorf("no state %s, which holds variables", s)
		}
	}
	for _, e := range d.Ends {
		if !has(e.Src) {
			return fmt.Errorf("no state %s, which an end edge leaves", e.Src)
		}
	}
	for _, e := range d.Edges {
		for _, s := range []string{e.Src, e.Dst} {
			if !has(s) {
				return fmt.Errorf("no state %s, which the edge on %s names", s, e.Event.Channel)
			}
		}
	}
	return nil
}

// wellFormed reports, as an error, a state d names but does not declare, or
// a clause of d that is not well formed (clause.WellFormed).
func (d *Diagram) wellFormed() error {
	if err := d.declared(); err != nil {
		return err
	}
	if err := clause.WellFormed(d.InitPost); err != nil {
		return fmt.Errorf("the start edge: %w", err)
	}
	for _, e := range d.Edges {
		for _, c := range []clause.And{e.Guard, e.Post} {
			if err := clause.WellFormed(c); err != nil {
				return fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
			}
		}
	}
	for _, e := range d.Ends {
		if err := clause.WellFormed(e.Guard); err != nil {
			return fmt.Errorf("the end edge from %s: %w", e.Src, err)
		}
	}
	return nil
}
