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
// end edges, which only promotion reads, as deletions.
type Diagram struct {
	States   []string
	Vars     map[string][]string
	Init     string
	InitPost clause.And
	Edges    []Edge
	Ends     []End
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
