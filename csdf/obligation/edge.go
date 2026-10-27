package obligation

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/logic"
)

// Step is the relation an edge of d states between the values of the state
// variables before it, the fields its event receives and the values after
// it: its guard and its post. Each received field is an argument of its own,
// named by the field (#69); a sent field is a value the step has already
// bound, a state variable or another field, and so no argument of its own.
func Step(d core.Diagram, e core.Edge) (logic.Formula, error) {
	if err := declared(d, e); err != nil {
		return nil, fmt.Errorf("obligation.Step: %w", err)
	}
	guard, err := Formula(e.Guard)
	if err != nil {
		return nil, fmt.Errorf("obligation.Step: the guard of %s -> %s: %w", e.Src, e.Dst, err)
	}
	post, err := Formula(e.Post)
	if err != nil {
		return nil, fmt.Errorf("obligation.Step: the post of %s -> %s: %w", e.Src, e.Dst, err)
	}
	return logic.Simplify(logic.And(guard, post)), nil
}

// Enabled is that an edge of d can be taken from the values of its source
// state: for some values of the fields its event receives, offered by the
// environment or, for a tau, chosen by the diagram, its guard holds and its
// post admits some values after it. Each variable it chooses has the type
// the diagram declares of it, the field's or the state variable's. A claim
// that some state satisfies it is sound only under the Lemmas of the edge's
// clauses.
func Enabled(d core.Diagram, e core.Edge) (logic.Formula, error) {
	step, err := Step(d, e)
	if err != nil {
		return nil, fmt.Errorf("obligation.Enabled: %w", err)
	}
	var chosen []logic.Var
	var types []string
	for i, f := range e.Event.Fields {
		if f.Send {
			continue
		}
		t, err := d.FieldType(e.Event, i)
		if err != nil {
			return nil, fmt.Errorf("obligation.Enabled: %w", err)
		}
		chosen, types = append(chosen, logic.Var(f.Name)), append(types, t.String())
	}
	for _, v := range d.Vars[e.Dst] {
		t, err := d.VarType(v)
		if err != nil {
			return nil, fmt.Errorf("obligation.Enabled: %w", err)
		}
		chosen, types = append(chosen, Var(clause.Free{Name: v, Primed: true})), append(types, t.String())
	}
	return logic.Simplify(logic.ExistsTyped(chosen, types, step)), nil
}

// declared checks that e names states of d and reads only what it binds: a
// guard the values of its source state and the fields of its event, never a
// value after the step; a post those, and the values after the step its
// destination holds. A received field named like a state variable would be
// bound over it, and is refused; a sent field is a state variable of the
// source or a field the event receives, or it would be free.
func declared(d core.Diagram, e core.Edge) error {
	for _, s := range []string{e.Src, e.Dst} {
		if !slices.Contains(d.States, s) {
			return fmt.Errorf("no state %s, which the edge %s -> %s names", s, e.Src, e.Dst)
		}
	}
	before := map[string]bool{}
	for _, v := range d.Vars[e.Src] {
		before[v] = true
	}
	for _, f := range e.Event.Fields {
		if !f.Send && core.ReceivesOver(f.Name, d.Vars[e.Src], d.Vars[e.Dst]) {
			return fmt.Errorf("the edge %s -> %s receives %s, which names a state variable", e.Src, e.Dst, f.Name)
		}
		if !f.Send {
			if before[f.Name] {
				return fmt.Errorf("the edge %s -> %s receives %s twice", e.Src, e.Dst, f.Name)
			}
			before[f.Name] = true
		}
	}
	for _, f := range e.Event.Fields {
		if f.Send && !before[f.Name] {
			return fmt.Errorf("the edge %s -> %s sends %s, which neither %s holds nor the event receives", e.Src, e.Dst, f.Name, e.Src)
		}
	}
	for _, v := range clause.FreeVars(e.Guard) {
		if v.Primed {
			return fmt.Errorf("the guard of %s -> %s reads %s', a value after the step", e.Src, e.Dst, v.Name)
		}
		if !before[v.Name] {
			return fmt.Errorf("the guard of %s -> %s reads %s, which neither %s holds nor the event gives", e.Src, e.Dst, v.Name, e.Src)
		}
	}
	for _, v := range clause.FreeVars(e.Post) {
		if v.Primed && !slices.Contains(d.Vars[e.Dst], v.Name) {
			return fmt.Errorf("the post of %s -> %s gives %s', which %s does not hold", e.Src, e.Dst, v.Name, e.Dst)
		}
		if !v.Primed && !before[v.Name] {
			return fmt.Errorf("the post of %s -> %s reads %s, which neither %s holds nor the event gives", e.Src, e.Dst, v.Name, e.Src)
		}
	}
	return nil
}
