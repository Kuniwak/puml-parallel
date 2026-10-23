// Package core is the constructors of docs/CLAUSE_DESIGN.md — composition,
// hiding and promotion — on diagrams whose clauses are in the csdf/clause
// representation, and the meaning they are tested against: a diagram
// expanded over finite types into an LTS (csdf/lts).
package core

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Tau is the channel of an internal event.
const Tau = "tau"

// Field is a field of an event: received (?Name) or sent (!Name).
type Field struct {
	Send bool
	Name string
}

// Dir is how a label writes f's direction: "!" for a field sent, "?" for
// one received.
func (f Field) Dir() string {
	if f.Send {
		return "!"
	}
	return "?"
}

// FieldOfDir is the field named name whose direction dir writes, as Dir
// does; any other dir is an error.
func FieldOfDir(dir, name string) (Field, error) {
	f, err := fieldOfDir(dir, name)
	if err != nil {
		return Field{}, fmt.Errorf("core.FieldOfDir: %w", err)
	}
	return f, nil
}

func fieldOfDir(dir, name string) (Field, error) {
	switch dir {
	case "?", "!":
		return Field{Send: dir == "!", Name: name}, nil
	}
	return Field{}, fmt.Errorf("the field %s has the direction %q, neither ? nor !", name, dir)
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

// Edge is a transition: Src --> Dst : Event ; Guard ; Post. Line is where
// the reader read it, from 1, for a reader of a report alone; it is outside
// the meaning of the edge, and 0 for an edge no reader read, such as one a
// constructor built.
type Edge struct {
	Src, Dst    string
	Event       Event
	Guard, Post clause.And
	Line        int
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
	// InitLine is where the reader read the start edge, as Edge.Line is.
	InitLine int
	Edges    []Edge
	Ends     []End
	Assumes  []Assumption
	// Families are, for a promoted diagram, the local states of the family
	// whose instances each map holds: what an instance in the map may be
	// (docs/PROMOTION.md). A diagram no promotion made has none.
	Families []FamilyStates
}

// FamilyStates is what an instance in the map Map may be: at one of
// States other than Init, holding that state's Vars as its fields. Init,
// the local start state, is absence: an instance there has no entry in
// the map, and it holds no variables.
type FamilyStates struct {
	Map    string
	Init   string
	States []string
	Vars   map[string][]string
}

// clone is f sharing no slice or map with it.
func (f FamilyStates) clone() FamilyStates {
	vars := make(map[string][]string, len(f.Vars))
	for s, vs := range f.Vars {
		vars[s] = slices.Clone(vs)
	}
	return FamilyStates{Map: f.Map, Init: f.Init, States: slices.Clone(f.States), Vars: vars}
}

// cloneFamilies are fs, each cloned.
func cloneFamilies(fs []FamilyStates) []FamilyStates {
	var out []FamilyStates
	for _, f := range fs {
		out = append(out, f.clone())
	}
	return out
}

// familiesWellFormed reports, as an error, a family of d of a map no state
// of d holds, two families of one map, or a family whose start state or
// variables are of no state of it, or whose start state holds variables.
func (d *Diagram) familiesWellFormed() error {
	seen := map[string]bool{}
	for _, f := range d.Families {
		if seen[f.Map] {
			return fmt.Errorf("two families of %s", f.Map)
		}
		seen[f.Map] = true
		held := false
		for _, vs := range d.Vars {
			held = held || slices.Contains(vs, f.Map)
		}
		if !held {
			return fmt.Errorf("the family of %s: %s is no variable of any state", f.Map, f.Map)
		}
		if !slices.Contains(f.States, f.Init) {
			return fmt.Errorf("the family of %s: its start state %s is none of its states", f.Map, f.Init)
		}
		for _, s := range slices.Sorted(maps.Keys(f.Vars)) {
			if !slices.Contains(f.States, s) {
				return fmt.Errorf("the family of %s: variables of %s, which is none of its states", f.Map, s)
			}
		}
		if len(f.Vars[f.Init]) > 0 {
			return fmt.Errorf("the family of %s: its start state %s, which is absence, holds variables", f.Map, f.Init)
		}
	}
	return nil
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
	if err := d.familiesWellFormed(); err != nil {
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

// clauses are every guard and post of d.
func (d *Diagram) clauses() []clause.And {
	fs := []clause.And{d.InitPost}
	for _, e := range d.Edges {
		fs = append(fs, e.Guard, e.Post)
	}
	for _, e := range d.Ends {
		fs = append(fs, e.Guard)
	}
	return fs
}

// Received are the names of the fields e receives, in order: those the
// environment offers, or for a tau the diagram chooses.
func (e Event) Received() []string {
	names := []string{}
	for _, f := range e.Fields {
		if !f.Send {
			names = append(names, f.Name)
		}
	}
	return names
}

// ReceivesOver reports whether a field named field, received by an edge,
// names a variable of one of the states vars lists, over which it would
// bind: a received field must be named apart from the variables of the
// states of its edge.
func ReceivesOver(field string, vars ...[]string) bool {
	for _, vs := range vars {
		if slices.Contains(vs, field) {
			return true
		}
	}
	return false
}

// ParseEvent reads an event written as a label is (Label), as
// SplitEvent takes it apart: a channel, then ?x for each field
// received and !x for each sent, in order, where x is the field's name or
// the value it carries. An x that is empty is refused; neither it nor the
// channel may hold ? or !.
func ParseEvent(text string) (Event, error) {
	channel, parts, err := SplitEvent(text, FieldNameOrValue)
	if err != nil {
		return Event{}, fmt.Errorf("core.ParseEvent: %w", err)
	}
	e := Event{Channel: channel}
	for _, p := range parts {
		e.Fields = append(e.Fields, Field{Send: p.Send, Name: p.Name})
	}
	return e, nil
}

// Label is e as a diagram writes it: its channel, then ?name for each field
// it receives and !name for each it sends, in order.
func (e Event) Label() string {
	var sb strings.Builder
	sb.WriteString(e.Channel)
	for _, f := range e.Fields {
		sb.WriteString(f.Dir())
		sb.WriteString(f.Name)
	}
	return sb.String()
}
