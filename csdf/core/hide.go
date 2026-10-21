package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Hide is d with the channels hidden (docs/CLAUSE_DESIGN.md, a
// constructor): ⟦Hide(d, C)⟧ is ⟦d⟧ \ C. An edge on a hidden channel
// becomes τ, and each field it received is bound by ∃ (O4): the step can be
// taken when some value satisfies the guard, ∃x. g, and leads where some
// value satisfies both, ∃x. (g ∧ p), so that the guard and the post agree
// on the value chosen. A sent field names a value already bound, so it is
// dropped.
func Hide(d *Diagram, channels []string) (*Diagram, error) {
	out := &Diagram{Vars: d.Vars, Init: d.Init, InitPost: d.InitPost}
	for _, e := range d.Edges {
		if !slices.Contains(channels, e.Event.Channel) {
			out.Edges = append(out.Edges, e)
			continue
		}
		guard, post := e.Guard, and(e.Guard, e.Post)
		for i := len(e.Event.Fields) - 1; i >= 0; i-- {
			f := e.Event.Fields[i]
			if f.Send {
				continue
			}
			var err error
			// Until fields carry types, a field's type is the one named
			// like it.
			if guard, err = bind(guard, f.Name, f.Name); err != nil {
				return nil, fmt.Errorf("core.Hide: %s: %w", e.Event.Channel, err)
			}
			if post, err = bind(post, f.Name, f.Name); err != nil {
				return nil, fmt.Errorf("core.Hide: %s: %w", e.Event.Channel, err)
			}
		}
		out.Edges = append(out.Edges, Edge{Src: e.Src, Dst: e.Dst, Event: Event{Channel: Tau}, Guard: guard, Post: post})
	}
	return out, nil
}

// bind is ∃name : typ. f.
func bind(f clause.And, name, typ string) (clause.And, error) {
	e, err := clause.Close(f, clause.Free{Name: name}, typ)
	if err != nil {
		return clause.And{}, err
	}
	return clause.And{Conjuncts: []clause.Formula{e}}, nil
}
