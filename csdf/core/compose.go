package core

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/types"

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
	if len(l.Ends) > 0 || len(r.Ends) > 0 {
		return nil, fmt.Errorf("core.Compose: a diagram with end edges is not composed yet")
	}
	r = renamedApart(l, r)
	channels, err := sharedChannels(l.Channels, r.Channels)
	if err != nil {
		return nil, fmt.Errorf("core.Compose: %w", err)
	}
	out := &Diagram{
		Vars:     map[string][]string{},
		Types:    mergedTypes(l.Types, r.Types),
		Channels: channels,
		Init:     pair(l.Init, r.Init),
		InitPost: Conj(l.InitPost, r.InitPost),
		Assumes:  append(slices.Clone(l.Assumes), r.Assumes...),
		Families: append(cloneFamilies(l.Families), cloneFamilies(r.Families)...),
	}
	for _, ls := range l.States {
		for _, rs := range r.States {
			lv, rv := l.Vars[ls], r.Vars[rs]
			s := pair(ls, rs)
			if slices.Contains(out.States, s) {
				return nil, fmt.Errorf("core.Compose: two pairs of states are both %s", s)
			}
			out.States = append(out.States, s)
			if out.Labels == nil {
				out.Labels = map[string]string{}
			}
			out.Labels[s] = "(" + l.label(ls) + ", " + r.label(rs) + ")"
			if vars := append(slices.Clone(lv), rv...); len(vars) > 0 {
				out.Vars[s] = vars
			}
		}
	}
	synced := func(c string) bool { return c != Tau && slices.Contains(sync, c) }
	for _, e := range l.Edges {
		if synced(e.Event.Channel) {
			continue
		}
		for _, rs := range r.States {
			out.Edges = append(out.Edges, alone(e, pair(e.Src, rs), pair(e.Dst, rs), r.Vars[rs]))
		}
	}
	for _, e := range r.Edges {
		if synced(e.Event.Channel) {
			continue
		}
		for _, ls := range l.States {
			out.Edges = append(out.Edges, alone(e, pair(ls, e.Src), pair(ls, e.Dst), l.Vars[ls]))
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

// renamedApart is r with each state variable that l also holds renamed to
// a fresh name made from its own, which neither side uses
// (docs/CLAUSE_DESIGN.md): the two are separate processes, so the two
// variables are two. A field of an edge named like the variable is that
// field, not the variable, and is left alone there.
func renamedApart(l, r *Diagram) *Diagram {
	held := func(d *Diagram) map[string]bool {
		m := map[string]bool{}
		for _, vs := range d.Vars {
			for _, v := range vs {
				m[v] = true
			}
		}
		return m
	}
	lv, rv := held(l), held(r)
	avoid := map[string]bool{}
	for _, d := range []*Diagram{l, r} {
		for v := range held(d) {
			avoid[v] = true
		}
		for _, f := range d.clauses() {
			for _, v := range clause.FreeVars(f) {
				avoid[v.Name] = true
			}
		}
		for _, e := range d.Edges {
			for _, f := range e.Event.Fields {
				avoid[f.Name] = true
			}
		}
	}
	rename := map[string]string{}
	for _, v := range slices.Sorted(maps.Keys(rv)) {
		if lv[v] {
			fresh := clause.Fresh(avoid, v)
			avoid[fresh] = true
			rename[v] = fresh
		}
	}
	if len(rename) == 0 {
		return r
	}
	in := func(f clause.And, fields []Field) clause.And {
		field := func(name string) bool {
			return slices.ContainsFunc(fields, func(x Field) bool { return x.Name == name })
		}
		return clause.MapFormula(f, 0, func(t clause.Term, _ int) clause.Term {
			v, ok := t.(clause.Free)
			if to, hit := rename[v.Name]; ok && hit && (v.Primed || !field(v.Name)) {
				return clause.Free{Name: to, Primed: v.Primed}
			}
			return t
		}).(clause.And)
	}
	out := *r
	out.Families = cloneFamilies(r.Families)
	for i, f := range out.Families {
		if to, ok := rename[f.Map]; ok {
			out.Families[i].Map = to
		}
	}
	out.Types = map[string]types.Type{}
	for v, t := range r.Types {
		if to, ok := rename[v]; ok {
			v = to
		}
		out.Types[v] = t
	}
	out.Vars = map[string][]string{}
	for s, vs := range r.Vars {
		for _, v := range vs {
			if to, ok := rename[v]; ok {
				v = to
			}
			out.Vars[s] = append(out.Vars[s], v)
		}
	}
	out.InitPost = in(r.InitPost, nil)
	out.Edges = make([]Edge, len(r.Edges))
	for i, e := range r.Edges {
		e.Guard, e.Post = in(e.Guard, e.Event.Fields), in(e.Post, e.Event.Fields)
		out.Edges[i] = e
	}
	return &out
}

// mergedTypes are the types of l and of r, which renamedApart has made
// disjoint; nil when neither has any.
func mergedTypes(l, r map[string]types.Type) map[string]types.Type {
	if len(l) == 0 && len(r) == 0 {
		return nil
	}
	out := maps.Clone(l)
	if out == nil {
		out = map[string]types.Type{}
	}
	maps.Copy(out, r)
	return out
}

// sharedChannels are the channels l and r declare; a channel both declare
// is one channel, so the two must declare it alike.
func sharedChannels(l, r map[string][]types.Type) (map[string][]types.Type, error) {
	if len(l) == 0 && len(r) == 0 {
		return nil, nil
	}
	out := map[string][]types.Type{}
	for c, ts := range l {
		out[c] = slices.Clone(ts)
	}
	for _, c := range slices.Sorted(maps.Keys(r)) {
		if lt, ok := l[c]; ok && !slices.EqualFunc(lt, r[c], types.Type.Equal) {
			return nil, fmt.Errorf("the channel %s is declared %s on one side and %s on the other", c, ChannelDecl(c, lt), ChannelDecl(c, r[c]))
		}
		out[c] = slices.Clone(r[c])
	}
	return out, nil
}

// ChannelDecl is the declaration of the channel c with fields of the types
// fields, as a channel note writes it.
func ChannelDecl(c string, fields []types.Type) string {
	s := c
	for i, t := range fields {
		sep := "."
		if i == 0 {
			sep = " : "
		}
		if t.IsMap() {
			s += sep + "(" + t.String() + ")"
		} else {
			s += sep + t.String()
		}
	}
	return s
}

// pair is the ID of the state where l is in l and r in r, as csdfparallel
// has always named it; its label says the pair.
func pair(l, r string) string { return l + "_" + r }

// Conj is the conjunction of fs, normalised.
func Conj(fs ...clause.And) clause.And {
	var cs []clause.Formula
	for _, f := range fs {
		cs = append(cs, f)
	}
	return clause.Normalize(clause.And{Conjuncts: cs})
}

// alone is e taken by its side while the other side, holding vars, stays.
func alone(e Edge, src, dst string, vars []string) Edge {
	return Edge{Src: src, Dst: dst, Event: e.Event, Guard: e.Guard, Post: Conj(e.Post, Unchanged(vars))}
}

// Unchanged says the variables vars are unchanged by the step.
func Unchanged(vars []string) clause.And {
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
		Guard: Conj(gl, gr, clause.And{Conjuncts: eqs}),
		Post:  Conj(pl, pr),
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
