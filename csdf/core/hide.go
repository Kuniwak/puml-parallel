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
	out := &Diagram{Name: d.Name, Labels: d.Labels, States: d.States, Vars: d.Vars, Types: d.Types, Init: d.Init, InitPost: d.InitPost, Ends: d.Ends, Assumes: d.Assumes, Families: cloneFamilies(d.Families)}
	for c, ts := range d.Channels {
		if slices.Contains(channels, c) {
			continue
		}
		if out.Channels == nil {
			out.Channels = map[string][]Type{}
		}
		out.Channels[c] = ts
	}
	for _, e := range d.Edges {
		if !slices.Contains(channels, e.Event.Channel) {
			out.Edges = append(out.Edges, e)
			continue
		}
		guard, post := e.Guard, Conj(e.Guard, e.Post)
		for i := len(e.Event.Fields) - 1; i >= 0; i-- {
			f := e.Event.Fields[i]
			if f.Send {
				continue
			}
			var err error
			typ := f.Name
			if ts, ok := d.Channels[e.Event.Channel]; ok && i < len(ts) {
				typ = ts[i].String()
			}
			if guard, err = Bind(guard, f.Name, typ); err != nil {
				return nil, fmt.Errorf("core.Hide: %s: %w", e.Event.Channel, err)
			}
			if post, err = Bind(post, f.Name, typ); err != nil {
				return nil, fmt.Errorf("core.Hide: %s: %w", e.Event.Channel, err)
			}
		}
		out.Edges = append(out.Edges, Edge{Src: e.Src, Dst: e.Dst, Event: Event{Channel: Tau}, Guard: guard, Post: post})
	}
	return out, nil
}

// Bind is ∃name : typ. f. When f does not read name, it is f itself: a
// binder of nothing says only that typ has a value, and every type a
// diagram ranges over has one; this also keeps the result well formed, as
// an ∃ with an empty body is not.
func Bind(f clause.And, name, typ string) (clause.And, error) {
	if !slices.Contains(clause.FreeVars(f), clause.Free{Name: name}) {
		return f, nil
	}
	e, err := clause.Close(f, clause.Free{Name: name}, typ)
	if err != nil {
		return clause.And{}, err
	}
	return clause.And{Conjuncts: []clause.Formula{e}}, nil
}
