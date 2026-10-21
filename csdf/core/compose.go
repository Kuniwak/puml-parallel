package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Compose is the interface parallel of l and r synchronised on the channels
// sync (docs/CLAUSE_DESIGN.md, a constructor): ⟦Compose(l, r)⟧ is
// ⟦l⟧ ‖ ⟦r⟧ with the alphabet of each its own channels. Two synchronised
// events agree field by field: a received field takes the name of the
// field it meets, by substitution into the holes of the side that received
// it, and two sent fields are equal, a builtin @eq. A substitution that
// would capture a name of that side is refused, as is a state variable both
// sides hold.
func Compose(l, r *Diagram, sync []string) (*Diagram, error) {
	out := &Diagram{
		Vars:     map[string][]string{},
		Init:     pair(l.Init, r.Init),
		InitPost: and(l.InitPost, r.InitPost),
	}
	for ls, lv := range l.Vars {
		for rs, rv := range r.Vars {
			for _, v := range lv {
				if slices.Contains(rv, v) {
					return nil, fmt.Errorf("core.Compose: both %s and %s hold %s", ls, rs, v)
				}
			}
			out.Vars[pair(ls, rs)] = append(append([]string{}, lv...), rv...)
		}
	}
	synced := func(c string) bool { return c != Tau && slices.Contains(sync, c) }
	for _, e := range l.Edges {
		if synced(e.Event.Channel) {
			continue
		}
		for rs, rv := range r.Vars {
			out.Edges = append(out.Edges, alone(e, pair(e.Src, rs), pair(e.Dst, rs), rv))
		}
	}
	for _, e := range r.Edges {
		if synced(e.Event.Channel) {
			continue
		}
		for ls, lv := range l.Vars {
			out.Edges = append(out.Edges, alone(e, pair(ls, e.Src), pair(ls, e.Dst), lv))
		}
	}
	for _, el := range l.Edges {
		if !synced(el.Event.Channel) {
			continue
		}
		for _, er := range r.Edges {
			if er.Event.Channel != el.Event.Channel {
				continue
			}
			e, err := together(el, er, l, r)
			if err != nil {
				return nil, fmt.Errorf("core.Compose: %w", err)
			}
			out.Edges = append(out.Edges, e)
		}
	}
	return out, nil
}

func pair(l, r string) string { return "(" + l + ", " + r + ")" }

func and(fs ...clause.And) clause.And {
	var cs []clause.Formula
	for _, f := range fs {
		cs = append(cs, f)
	}
	return clause.Normalize(clause.And{Conjuncts: cs})
}

// alone is e taken by its side while the other side, holding vars, stays.
func alone(e Edge, src, dst string, vars []string) Edge {
	return Edge{Src: src, Dst: dst, Event: e.Event, Guard: e.Guard, Post: and(e.Post, frame(vars))}
}

// frame says the variables vars are unchanged by the step.
func frame(vars []string) clause.And {
	var cs []clause.Formula
	for _, v := range vars {
		cs = append(cs, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: v, Primed: true}, clause.Free{Name: v}}})
	}
	return clause.And{Conjuncts: cs}
}

func together(el, er Edge, l, r *Diagram) (Edge, error) {
	fl, fr := el.Event.Fields, er.Event.Fields
	if len(fl) != len(fr) {
		return Edge{}, fmt.Errorf("%s has %d fields on one side and %d on the other", el.Event.Channel, len(fl), len(fr))
	}
	renameL, renameR := map[string]string{}, map[string]string{}
	var eqs []clause.Formula
	fields := make([]Field, len(fl))
	for i := range fl {
		a, b := fl[i], fr[i]
		switch {
		case !a.Send && !b.Send:
			renameR[b.Name] = a.Name
			fields[i] = a
		case a.Send && !b.Send:
			renameR[b.Name] = a.Name
			fields[i] = a
		case !a.Send && b.Send:
			renameL[a.Name] = b.Name
			fields[i] = b
		default:
			eqs = append(eqs, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: a.Name}, clause.Free{Name: b.Name}}})
			fields[i] = a
		}
	}
	gl, pl, err := renamed(el, l, renameL)
	if err != nil {
		return Edge{}, err
	}
	gr, pr, err := renamed(er, r, renameR)
	if err != nil {
		return Edge{}, err
	}
	return Edge{
		Src:   pair(el.Src, er.Src),
		Dst:   pair(el.Dst, er.Dst),
		Event: Event{Channel: el.Event.Channel, Fields: fields},
		Guard: and(gl, gr, clause.And{Conjuncts: eqs}),
		Post:  and(pl, pr),
	}, nil
}

// renamed is the guard and post of e with each received field renamed, at
// once, as rename says; a new name that e's side already uses for a state
// variable, or for another field of e, would be captured.
func renamed(e Edge, d *Diagram, rename map[string]string) (clause.And, clause.And, error) {
	taken := map[string]bool{}
	for _, v := range d.Vars[e.Src] {
		taken[v] = true
	}
	for _, v := range d.Vars[e.Dst] {
		taken[v] = true
	}
	for _, f := range e.Event.Fields {
		if _, ok := rename[f.Name]; !ok {
			taken[f.Name] = true
		}
	}
	for from, to := range rename {
		if from != to && taken[to] {
			return clause.And{}, clause.And{}, fmt.Errorf("%s: renaming %s to %s would capture %s", e.Event.Channel, from, to, to)
		}
	}
	sub := func(f clause.And) clause.And {
		return clause.Normalize(clause.MapFormula(f, 0, func(t clause.Term, _ int) clause.Term {
			if v, ok := t.(clause.Free); ok && !v.Primed {
				if to, ok := rename[v.Name]; ok {
					return clause.Free{Name: to}
				}
			}
			return t
		}))
	}
	return sub(e.Guard), sub(e.Post), nil
}
