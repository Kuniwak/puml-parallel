package obligation

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
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
// post admits some values after it.
func Enabled(d core.Diagram, e core.Edge) (logic.Formula, error) {
	step, err := Step(d, e)
	if err != nil {
		return nil, fmt.Errorf("obligation.Enabled: %w", err)
	}
	var chosen []logic.Var
	for _, f := range e.Event.Fields {
		if !f.Send {
			chosen = append(chosen, logic.Var(f.Name))
		}
	}
	for _, v := range d.Vars[e.Dst] {
		chosen = append(chosen, Var(clause.Free{Name: v, Primed: true}))
	}
	return logic.Simplify(logic.Exists(chosen, step)), nil
}

func declared(d core.Diagram, e core.Edge) error {
	for _, s := range []string{e.Src, e.Dst} {
		if !slices.Contains(d.States, s) {
			return fmt.Errorf("no state %s, which the edge %s -> %s names", s, e.Src, e.Dst)
		}
	}
	return nil
}
