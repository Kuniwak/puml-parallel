package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Family is a promoted family: its instances live in the map Map, an
// instance ID is written ID, and an instance runs Local, whose start state
// means the instance does not exist.
type Family struct {
	Map, ID string
	Local   *Diagram
}

// Target is one occurrence in a sync: an instance of the family of Map,
// bound to the ID variable ID, taking the local event Event, or the sync's
// own event when Event is empty.
type Target struct {
	Map, ID, Event string
}

func (t Target) event(s Sync) string {
	if t.Event == "" {
		return s.Event
	}
	return t.Event
}

// Sync is a global event Event taken by its Targets together.
type Sync struct {
	Event   string
	Targets []Target
}

// Promotion is a global diagram of one state, State, holding the maps of its
// Families, with Syncs among them.
type Promotion struct {
	State    string
	Families []Family
	Syncs    []Sync
}

// Promote is the global diagram of p (docs/CLAUSE_DESIGN.md, a constructor):
// ⟦Promote(p)⟧ is the instances of every family interleaved, each running
// its local diagram with its own events renamed e to e.i, synchronised on
// the syncs. A local variable v of an instance bound to x is the term
// m[x].v: promotion substitutes it into the holes of the local clauses (O3),
// so that two instances of one family in one sync are told apart, and the
// sync says their IDs differ (@ne). Two moves of one map in one step chain
// through the intermediate map, bound by ∃.
func Promote(p Promotion) (*Diagram, error) {
	out := &Diagram{Init: p.State, Vars: map[string][]string{}}
	var maps []string
	var start []clause.Formula
	for _, f := range p.Families {
		if len(f.Local.Vars[f.Local.Init]) > 0 {
			return nil, fmt.Errorf("core.Promote: %s: the start state %s holds variables, but it means the instance does not exist", f.Map, f.Local.Init)
		}
		maps = append(maps, f.Map)
		start = append(start, clause.Builtin{Op: clause.OpAll, Args: []clause.Term{clause.Free{Name: f.Map, Primed: true}}, State: f.Local.Init})
	}
	out.Vars[p.State] = maps
	out.InitPost = clause.And{Conjuncts: start}
	taken := map[string]bool{}
	for _, s := range p.Syncs {
		edges, err := promoteSync(p, s)
		if err != nil {
			return nil, fmt.Errorf("core.Promote: sync %s: %w", s.Event, err)
		}
		out.Edges = append(out.Edges, edges...)
		for _, tg := range s.Targets {
			taken[tg.Map+"."+tg.event(s)] = true
		}
	}
	for _, f := range p.Families {
		for _, e := range f.Local.Edges {
			if taken[f.Map+"."+e.Event.Channel] {
				continue
			}
			edge, err := promoteAlone(p, f, e)
			if err != nil {
				return nil, fmt.Errorf("core.Promote: %s: %w", f.Map, err)
			}
			out.Edges = append(out.Edges, edge)
		}
	}
	return out, nil
}

// move is one instance's part in a step: the local edge e of the family f,
// taken by the instance bound to id.
type move struct {
	f  Family
	id string
	e  Edge
}

func promoteAlone(p Promotion, f Family, e Edge) (Edge, error) {
	if slices.ContainsFunc(e.Event.Fields, func(fl Field) bool { return fl.Name == f.ID }) {
		return Edge{}, fmt.Errorf("%s has a field named like the ID %s", e.Event.Channel, f.ID)
	}
	guard, post, err := step(p, []move{{f: f, id: f.ID, e: e}}, map[string]bool{f.ID: true})
	if err != nil {
		return Edge{}, err
	}
	if e.Event.Channel == Tau {
		// An internal event has no argument: the instance is any that can
		// take it (docs/PROMOTION.md).
		if guard, err = bind(guard, f.ID, f.ID); err != nil {
			return Edge{}, err
		}
		if post, err = bind(and(guard, post), f.ID, f.ID); err != nil {
			return Edge{}, err
		}
		return Edge{Src: p.State, Dst: p.State, Event: Event{Channel: Tau}, Guard: guard, Post: post}, nil
	}
	fields := append([]Field{{Name: f.ID}}, e.Event.Fields...)
	return Edge{Src: p.State, Dst: p.State, Event: Event{Channel: e.Event.Channel, Fields: fields}, Guard: guard, Post: post}, nil
}

func promoteSync(p Promotion, s Sync) ([]Edge, error) {
	if s.Event == Tau {
		return nil, fmt.Errorf("tau is internal and cannot be synchronised")
	}
	ids := map[string]bool{}
	for i, a := range s.Targets {
		ids[a.ID] = true
		for _, b := range s.Targets[:i] {
			if a.Map == b.Map && a.ID == b.ID {
				return nil, fmt.Errorf("%s(%s) is named twice: one instance plays one role", a.Map, a.ID)
			}
			if a.Map == b.Map && a.event(s) == b.event(s) {
				return nil, fmt.Errorf("%s takes its local event %s twice", a.Map, a.event(s))
			}
		}
	}
	var fields []Field
	for _, tg := range s.Targets {
		fields = append(fields, Field{Name: tg.ID})
	}
	// Every combination of the local edges the targets take.
	combos := [][]move{nil}
	for _, tg := range s.Targets {
		f, err := familyOf(p, tg.Map)
		if err != nil {
			return nil, err
		}
		var grown [][]move
		for _, e := range f.Local.Edges {
			if e.Event.Channel != tg.event(s) {
				continue
			}
			for _, c := range combos {
				grown = append(grown, append(slices.Clone(c), move{f: f, id: tg.ID, e: e}))
			}
		}
		combos = grown
	}
	var distinct []clause.Formula
	for i, a := range s.Targets {
		for _, b := range s.Targets[:i] {
			if a.Map == b.Map {
				distinct = append(distinct, clause.Builtin{Op: clause.OpNe, Args: []clause.Term{clause.Free{Name: b.ID}, clause.Free{Name: a.ID}}})
			}
		}
	}
	var out []Edge
	for _, c := range combos {
		merged := slices.Clone(fields)
		for _, m := range c {
			for _, fl := range m.e.Event.Fields {
				if ids[fl.Name] {
					return nil, fmt.Errorf("%s has a field named like the ID %s", m.e.Event.Channel, fl.Name)
				}
				if !slices.ContainsFunc(merged, func(g Field) bool { return g.Name == fl.Name }) {
					merged = append(merged, fl)
				}
			}
		}
		guard, post, err := step(p, c, ids)
		if err != nil {
			return nil, err
		}
		out = append(out, Edge{
			Src: p.State, Dst: p.State,
			Event: Event{Channel: s.Event, Fields: merged},
			Guard: and(clause.And{Conjuncts: distinct}, guard),
			Post:  post,
		})
	}
	return out, nil
}

func familyOf(p Promotion, m string) (Family, error) {
	for _, f := range p.Families {
		if f.Map == m {
			return f, nil
		}
	}
	return Family{}, fmt.Errorf("no family lives in %s", m)
}

// step is the guard and post of the moves taken together. Each move reads
// its instance in the map before the step; the moves of one map chain, each
// writing the map the next one starts from, the intermediate maps bound by
// ∃; a map no move writes is unchanged.
func step(p Promotion, moves []move, ids map[string]bool) (clause.And, clause.And, error) {
	avoid := map[string]bool{}
	for id := range ids {
		avoid[id] = true
	}
	for _, f := range p.Families {
		avoid[f.Map] = true
		for _, vars := range f.Local.Vars {
			for _, v := range vars {
				avoid[v] = true
			}
		}
	}
	var guards, posts []clause.Formula
	var bound []clause.Free // the intermediate maps, with the map each stands for
	var boundMap []string
	for _, f := range p.Families {
		var mine []move
		for _, m := range moves {
			if m.f.Map == f.Map {
				mine = append(mine, m)
			}
		}
		if len(mine) == 0 {
			posts = append(posts, frame([]string{f.Map}))
			continue
		}
		before := clause.Term(clause.Free{Name: f.Map})
		for k, m := range mine {
			after := clause.Term(clause.Free{Name: f.Map, Primed: true})
			if k < len(mine)-1 {
				v := clause.Free{Name: clause.Fresh(avoid, f.Map)}
				avoid[v.Name] = true
				bound, boundMap = append(bound, v), append(boundMap, f.Map)
				after = v
			}
			g, q, err := local(f, m, before, after)
			if err != nil {
				return clause.And{}, clause.And{}, err
			}
			guards, posts = append(guards, g), append(posts, q)
			before = after
		}
	}
	post := clause.Normalize(clause.And{Conjuncts: posts})
	for i := len(bound) - 1; i >= 0; i-- {
		// An intermediate map ranges over the type of the map it stands
		// for.
		e, err := clause.Close(post, bound[i], boundMap[i])
		if err != nil {
			return clause.And{}, clause.And{}, err
		}
		post = clause.And{Conjuncts: []clause.Formula{e}}
	}
	return clause.Normalize(clause.And{Conjuncts: guards}), post, nil
}

// local is the guard and post of one move, with the instance's local
// variables substituted: v is m[x].v in the map before the step (the moves
// of one step are of different instances, so every intermediate map agrees
// with m at x), v' is after[x].v.
func local(f Family, m move, before, after clause.Term) (clause.And, clause.And, error) {
	x := clause.Free{Name: m.id}
	vars := map[string]bool{}
	for _, v := range f.Local.Vars[m.e.Src] {
		vars[v] = true
	}
	for _, v := range f.Local.Vars[m.e.Dst] {
		vars[v] = true
	}
	now := clause.Term(clause.Free{Name: f.Map})
	sub := func(c clause.And) clause.And {
		return clause.Normalize(clause.MapFormula(c, 0, func(t clause.Term, _ int) clause.Term {
			v, ok := t.(clause.Free)
			if !ok || !vars[v.Name] {
				return t
			}
			if v.Primed {
				return clause.Field{Of: clause.Index{Map: after, Key: x}, Name: v.Name}
			}
			return clause.Field{Of: clause.Index{Map: now, Key: x}, Name: v.Name}
		}))
	}
	guard := and(clause.And{Conjuncts: []clause.Formula{
		clause.Builtin{Op: clause.OpAt, Args: []clause.Term{clause.Index{Map: now, Key: x}}, State: m.e.Src},
	}}, sub(m.e.Guard))
	post := and(clause.And{Conjuncts: []clause.Formula{
		clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{after, before, x}},
		clause.Builtin{Op: clause.OpAt, Args: []clause.Term{clause.Index{Map: after, Key: x}}, State: m.e.Dst},
	}}, sub(m.e.Post))
	return guard, post, nil
}
