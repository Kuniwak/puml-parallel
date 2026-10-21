// Package core is the constructors of docs/CLAUSE_DESIGN.md — composition,
// hiding and promotion — on diagrams whose clauses are in the csdf/clause
// representation, and the meaning they are tested against: a diagram
// expanded over finite types into an LTS (csdf/lts).
package core

import "github.com/Kuniwak/puml-parallel/csdf/clause"

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

// Diagram is a diagram whose clauses are formulas. Vars gives the state
// variables of each state; Init is the state the start edge leads to and
// InitPost the post of that edge.
type Diagram struct {
	Vars     map[string][]string
	Init     string
	InitPost clause.And
	Edges    []Edge
}
