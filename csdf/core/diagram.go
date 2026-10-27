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

	"github.com/Kuniwak/puml-parallel/v3/csdf/types"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/ident"
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
	Name   string
	Labels map[string]string
	States []string
	Vars   map[string][]string
	// Types are the types the state lines declare, of each variable one
	// for the whole diagram (docs/CLAUSE_DESIGN.md, Types).
	Types map[string]types.Type
	// Channels are the channels the channel notes declare, each with the
	// types of its fields in order.
	Channels map[string][]types.Type
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
// the map, and it holds no variables. Key is the ID parameter of the
// promote directive (Family.ID): the name whose type is the domain of the
// map, the type of the IDs that key it.
type FamilyStates struct {
	Map    string
	Key    string
	Init   string
	States []string
	Vars   map[string][]string
	// KeyType is the type of the IDs (Family.Key), and Types the types of
	// the variables of the local states.
	KeyType types.Type
	Types   map[string]types.Type
}

// clone is f sharing no slice or map with it.
func (f FamilyStates) clone() FamilyStates {
	vars := make(map[string][]string, len(f.Vars))
	for s, vs := range f.Vars {
		vars[s] = slices.Clone(vs)
	}
	return FamilyStates{Map: f.Map, Key: f.Key, Init: f.Init, States: slices.Clone(f.States), Vars: vars, KeyType: f.KeyType, Types: maps.Clone(f.Types)}
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
		if f.Key == "" {
			return fmt.Errorf("the family of %s: no key names the IDs of its map", f.Map)
		}
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

// namesWellFormed reports, as an error, a name d gives that the identifier
// rule refuses (docs/SYNTAX.md): a state, a variable, a channel, a field,
// an assumption, or a family's map, key or local state; or an event that
// repeats a field name (FieldsDistinct).
func (d *Diagram) namesWellFormed() error {
	if err := ident.CheckAll(d.States...); err != nil {
		return fmt.Errorf("a state: %w", err)
	}
	for _, vs := range d.Vars {
		if err := ident.CheckAll(vs...); err != nil {
			return fmt.Errorf("a variable: %w", err)
		}
	}
	for _, e := range d.Edges {
		if err := ident.Check(e.Event.Channel); err != nil {
			return fmt.Errorf("%s --> %s: the channel: %w", e.Src, e.Dst, err)
		}
		for _, f := range e.Event.Fields {
			if err := ident.Check(f.Name); err != nil {
				return fmt.Errorf("%s --> %s : %s: a field: %w", e.Src, e.Dst, e.Event.Channel, err)
			}
		}
		if err := e.Event.fieldsDistinct(); err != nil {
			return fmt.Errorf("%s --> %s: %w", e.Src, e.Dst, err)
		}
	}
	for _, a := range d.Assumes {
		if err := ident.Check(a.ID); err != nil {
			return fmt.Errorf("an assumption: %w", err)
		}
	}
	for _, f := range d.Families {
		if err := ident.CheckAll(append([]string{f.Map, f.Key, f.Init}, f.States...)...); err != nil {
			return fmt.Errorf("the family of %s: %w", f.Map, err)
		}
		for _, vs := range f.Vars {
			if err := ident.CheckAll(vs...); err != nil {
				return fmt.Errorf("the family of %s: a variable: %w", f.Map, err)
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
	if err := d.namesWellFormed(); err != nil {
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
	if err := d.atsWellFormed(); err != nil {
		return err
	}
	return d.NamesBound()
}

// NamesBound reports, as an error, a free name a clause of d reads that
// nothing binds (docs/CLAUSE_DESIGN.md, "Literals"): before the step a
// state variable of the source or a field of the event, after it a state
// variable of the target. A variable ∃ binds is no free name, and an
// environment function is applied, never read as a name; so a name that
// is none of these is a slip, such as a literal written as a name, never
// a free parameter.
func (d *Diagram) NamesBound() error {
	start := scope{src: "before the start", dst: "of " + d.Init, after: d.Vars[d.Init]}
	if err := start.check("post", d.InitPost); err != nil {
		return fmt.Errorf("the start edge: %w", err)
	}
	for _, e := range d.Edges {
		sc := scope{src: "of " + e.Src, before: d.Vars[e.Src], dst: "of " + e.Dst, after: d.Vars[e.Dst]}
		for _, f := range e.Event.Fields {
			sc.fields = append(sc.fields, f.Name)
		}
		if err := sc.check("guard", e.Guard); err != nil {
			return fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
		if err := sc.check("post", e.Post); err != nil {
			return fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
	}
	for _, e := range d.Ends {
		sc := scope{src: "of " + e.Src, before: d.Vars[e.Src], dst: "after the end"}
		if err := sc.check("guard", e.Guard); err != nil {
			return fmt.Errorf("the end edge from %s: %w", e.Src, err)
		}
	}
	return nil
}

// scope is what binds the free names of a clause: before the step the
// variables before of the state src names and the fields of the event,
// after it the variables after of the state dst names.
type scope struct {
	src, dst      string
	before, after []string
	fields        []string
}

// check is NamesBound of the clause c, what it is.
func (sc scope) check(what string, c clause.And) error {
	for _, v := range clause.FreeVars(c) {
		if v.Primed {
			if !slices.Contains(sc.after, v.Name) {
				return fmt.Errorf("the %s reads %s', which is no state variable %s", what, v.Name, sc.dst)
			}
			continue
		}
		if !slices.Contains(sc.before, v.Name) && !slices.Contains(sc.fields, v.Name) {
			return fmt.Errorf("the %s reads %s, which is no state variable %s, no field of the event and bound by no ∃", what, v.Name, sc.src)
		}
	}
	return nil
}

// atsWellFormed reports, as an error, an @at of an entry of a family's map
// at a state that is none of the family's, or at its start state, which is
// absence (said with @notin).
func (d *Diagram) atsWellFormed() error {
	if err := d.atsIn(d.InitPost); err != nil {
		return fmt.Errorf("the start edge: %w", err)
	}
	for _, e := range d.Edges {
		for _, c := range []clause.And{e.Guard, e.Post} {
			if err := d.atsIn(c); err != nil {
				return fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
			}
		}
	}
	for _, e := range d.Ends {
		if err := d.atsIn(e.Guard); err != nil {
			return fmt.Errorf("the end edge from %s: %w", e.Src, err)
		}
	}
	return nil
}

// atsIn is atsWellFormed of the clause c.
func (d *Diagram) atsIn(c clause.And) error {
	for _, f := range c.Conjuncts {
		switch f := f.(type) {
		case clause.Exists:
			if err := d.atsIn(f.Body); err != nil {
				return err
			}
		case clause.Builtin:
			if f.Op != clause.OpAt || len(f.Args) == 0 {
				continue
			}
			ix, ok := f.Args[0].(clause.Index)
			if !ok {
				continue
			}
			m, ok := ix.Map.(clause.Free)
			if !ok {
				continue
			}
			i := slices.IndexFunc(d.Families, func(x FamilyStates) bool { return x.Map == m.Name })
			if i < 0 {
				continue
			}
			fam := d.Families[i]
			if !slices.Contains(fam.States, f.State) {
				prime := ""
				if m.Primed {
					prime = "'"
				}
				return fmt.Errorf("@at(%s%s[…], %s), but %s is no state of the family of %s", m.Name, prime, f.State, f.State, m.Name)
			}
			if f.State == fam.Init {
				return fmt.Errorf("%s is the start state of the family of %s, which is absence; say @notin", f.State, m.Name)
			}
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

// fieldsDistinct is FieldsDistinct of e's fields.
func (e Event) fieldsDistinct() error {
	parts := make([]EventField, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = EventField{Send: f.Send, Name: f.Name}
	}
	return FieldsDistinct(e.Label(), parts)
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

// UnnamedAfter are the variables of vars that post does not name after the
// step (clause.PrimedNames), in the order of vars: constrained by nothing,
// each may be any value of its type after the step.
func UnnamedAfter(post clause.And, vars []string) []string {
	named := clause.PrimedNames(post)
	var free []string
	for _, v := range vars {
		if !slices.Contains(named, v) {
			free = append(free, v)
		}
	}
	return free
}

// NotKept are the state variables vars that e's guard reads and e's post
// does not keep, in the order the guard reads them. Any other name the
// guard reads is not the state's: a field of the event takes whatever value
// the next event carries, so the same event is enabled again when the
// state is kept, and a constant cannot change.
func (e Edge) NotKept(vars []string) []string {
	mutable := map[string]bool{}
	for _, v := range vars {
		mutable[v] = true
	}
	kept := map[string]bool{}
	for _, c := range clause.Normalize(e.Post).Conjuncts {
		if b, ok := c.(clause.Builtin); ok {
			if v, ok := clause.Keeps(b); ok {
				kept[v] = true
			}
		}
	}
	var loose []string
	for _, v := range clause.FreeVars(e.Guard) {
		if mutable[v.Name] && !kept[v.Name] && !slices.Contains(loose, v.Name) {
			loose = append(loose, v.Name)
		}
	}
	return loose
}
