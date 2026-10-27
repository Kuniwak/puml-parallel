package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// TypesDeclared reports, as an error, what the declarations of d leave
// out or contradict (docs/CLAUSE_DESIGN.md, Types): a state variable with
// no type; a channel whose events carry fields but which no channel note
// declares; an event with another number of fields than its channel; a
// variable sent on a field of another type, unless the field is of the
// type any; and a declared tau, which is internal.
func TypesDeclared(d *Diagram) error {
	for _, s := range d.States {
		for _, v := range d.Vars[s] {
			if _, ok := d.Types[v]; !ok {
				return fmt.Errorf("the variable %s of %s has no type; declare it as %s : %s ; <type>, or %s : %s ; any for a value of any type", v, s, s, v, s, v)
			}
		}
	}
	if _, ok := d.Channels[Tau]; ok {
		return fmt.Errorf("tau is internal and has no fields; remove its declaration from the channel note")
	}
	for _, e := range d.Edges {
		if err := e.typed(d); err != nil {
			return err
		}
	}
	return nil
}

// DeclareType records in ts that v is of the type t; a variable declared
// of another type before is an error, a variable having one type in a
// diagram.
func DeclareType(ts map[string]types.Type, v string, t types.Type) error {
	if u, ok := ts[v]; ok && !u.Equal(t) {
		return fmt.Errorf("the variable %s is declared of the type %s and of the type %s; a variable has one type in a diagram", v, u, t)
	}
	ts[v] = t
	return nil
}

// typed reports, as an error, a field of e that the declarations of d do
// not type, or a variable it sends of another type than its field.
func (e Edge) typed(d *Diagram) error {
	name := fmt.Sprintf("%s --> %s : %s", e.Src, e.Dst, e.Event.Label())
	decl, ok := d.Channels[e.Event.Channel]
	if err := e.Event.fieldsDistinct(); err != nil {
		return fmt.Errorf("%s --> %s: %w", e.Src, e.Dst, err)
	}
	switch {
	case e.Event.Channel == Tau:
		return nil
	case !ok && len(e.Event.Fields) > 0:
		return fmt.Errorf("%s: the channel %s carries fields, and no channel note declares their types; declare it as channel %s : <type>", name, e.Event.Channel, e.Event.Channel)
	case !ok:
		return nil
	case len(decl) != len(e.Event.Fields):
		return fmt.Errorf("%s has %d fields, but the channel %s is declared with %d", name, len(e.Event.Fields), e.Event.Channel, len(decl))
	}
	for i, f := range e.Event.Fields {
		// A sent field is a state variable of the source: no other field of
		// the event bears its name (fieldsDistinct, above), and NamesBound refuses
		// any other name.
		if !f.Send || !slices.Contains(d.Vars[e.Src], f.Name) {
			continue
		}
		t := d.Types[f.Name]
		if !decl[i].IsAny() && !t.Equal(decl[i]) {
			return fmt.Errorf("%s sends %s, of the type %s, on a field of the type %s", name, f.Name, t, decl[i])
		}
	}
	return nil
}

// VarType is the type d declares of its state variable v; one it does not
// declare is an error.
func (d *Diagram) VarType(v string) (types.Type, error) {
	t, ok := d.Types[v]
	if !ok {
		return types.Type{}, fmt.Errorf("the variable %s has no type", v)
	}
	return t, nil
}

// FieldType is the type d declares of the i-th field of e, by its position
// on e's channel; a channel that declares none there is an error.
func (d *Diagram) FieldType(e Event, i int) (types.Type, error) {
	decl, ok := d.Channels[e.Channel]
	if !ok || i < 0 || i >= len(decl) || i >= len(e.Fields) {
		name := "?"
		if 0 <= i && i < len(e.Fields) {
			name = e.Fields[i].Name
		}
		return types.Type{}, fmt.Errorf("%s carries %s, but declares no type of it", e.Channel, name)
	}
	return decl[i], nil
}
