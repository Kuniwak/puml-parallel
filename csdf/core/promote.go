package core

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Family is a promoted family: its instances live in the map Map, an
// instance ID is written ID, and an instance runs Local. Local's start state
// means that the instance does not exist (docs/PROMOTION.md, the
// local-diagram contract): it holds no state variables and has no
// self-loop, an edge out of it creates an instance, an edge into it deletes
// one, and Local has no end edge. An instance that does not exist has no
// entry in the map.
type Family struct {
	Map, ID string
	Local   *Diagram
}

// Target is one occurrence in a sync: an instance of the family of Map,
// bound to the ID variable ID, taking the local event Event.
type Target struct {
	Map, ID, Event string
}

// Sync is a global event Event taken by its Targets together. Guard is a
// clause of the global diagram on it, over the maps and the IDs of the
// targets: what it reads of a bound instance is carried by the event
// (docs/CLAUSE_DESIGN.md).
type Sync struct {
	Event   string
	Guard   clause.And
	Targets []Target
}

// Constrain is a guard conjoined onto every generated edge that matches
// Event in channel and the directions of its fields (docs/PROMOTION.md). Its
// field names are what Guard reads, beside the maps.
type Constrain struct {
	Event Event
	Guard clause.And
}

// Promotion is a global diagram whose states, its Modes, hold the maps of
// its Families, Init being the mode the start edge leads to, with Syncs
// among them and Constrains on them.
type Promotion struct {
	Modes      []string
	Init       string
	Families   []Family
	Syncs      []Sync
	Constrains []Constrain
}

// Promote is the global diagram of p (docs/CLAUSE_DESIGN.md, a constructor):
// ⟦Promote(p)⟧ is the instances of every family interleaved, each running
// its local diagram with its own events renamed e to e.i, synchronised on
// the syncs, each instance at its local start state, absent from the map,
// until created and after it is deleted. The posts of Local's start edge
// and of a deletion say nothing, there being no instance for them to be
// about, and are dropped with a warning (docs/PROMOTION.md). A
// local variable v of an instance bound to x is the term
// m[x].v: promotion substitutes it into the holes of the local clauses (O3),
// so that two instances of one family in one sync are told apart, and the
// sync says their IDs differ (@ne). Two moves of one map in one step chain
// through the intermediate map, bound by ∃.
func Promote(p Promotion) (*Diagram, []Warning, error) {
	out := &Diagram{States: slices.Clone(p.Modes), Init: p.Init, Vars: map[string][]string{}}
	var maps []string
	var start []clause.Formula
	var warnings []Warning
	for _, f := range p.Families {
		if err := f.Local.wellFormed(); err != nil {
			return nil, nil, fmt.Errorf("core.Promote: %s: %w", f.Map, err)
		}
		if err := lifecycle(f); err != nil {
			return nil, nil, fmt.Errorf("core.Promote: %s: %w", f.Map, err)
		}
		warnings = append(warnings, dropped(f)...)
		maps = append(maps, f.Map)
		start = append(start, clause.Builtin{Op: clause.OpEmpty, Args: []clause.Term{clause.Free{Name: f.Map, Primed: true}}})
	}
	for _, mode := range p.Modes {
		out.Vars[mode] = maps
	}
	out.InitPost = clause.And{Conjuncts: start}
	taken := map[string]bool{}
	for _, s := range p.Syncs {
		if err := clause.WellFormed(s.Guard); err != nil {
			return nil, nil, fmt.Errorf("core.Promote: sync %s: %w", s.Event, err)
		}
		for _, mode := range p.Modes {
			edges, err := promoteSync(p, s, mode)
			if err != nil {
				return nil, nil, fmt.Errorf("core.Promote: sync %s: %w", s.Event, err)
			}
			out.Edges = append(out.Edges, edges...)
		}
		for _, tg := range s.Targets {
			taken[tg.Map+"."+tg.Event] = true
		}
	}
	for _, f := range p.Families {
		var alone []move
		for _, e := range f.Local.Edges {
			if !taken[f.Map+"."+e.Event.Channel] {
				alone = append(alone, moveOf(f, f.ID, e))
			}
		}
		for _, mode := range p.Modes {
			for _, m := range alone {
				edge, err := promoteAlone(p, m, mode)
				if err != nil {
					return nil, nil, fmt.Errorf("core.Promote: %s: %w", f.Map, err)
				}
				out.Edges = append(out.Edges, edge)
			}
		}
	}
	if err := constrain(p, out.Edges); err != nil {
		return nil, nil, fmt.Errorf("core.Promote: %w", err)
	}
	return out, warnings, nil
}

// constrain conjoins each constrain of p onto every edge that matches its
// event in channel and field directions (docs/CLAUSE_DESIGN.md: the
// expansion of a monitor process in parallel).
// renamedTo is c's guard with each of c's fields renamed to the field of e
// at its position, all at once (docs/CLAUSE_DESIGN.md: a constrain matches by
// position).
func renamedTo(c Constrain, e Event) clause.And {
	to := map[string]string{}
	for i, fl := range c.Event.Fields {
		to[fl.Name] = e.Fields[i].Name
	}
	return clause.Normalize(clause.MapFormula(c.Guard, 0, func(u clause.Term, _ int) clause.Term {
		if v, ok := u.(clause.Free); ok {
			if n, ok := to[v.Name]; ok {
				return clause.Free{Name: n, Primed: v.Primed}
			}
		}
		return u
	}))
}

func constrain(p Promotion, edges []Edge) error {
	mapNames := map[string]bool{}
	for _, f := range p.Families {
		mapNames[f.Map] = true
	}
	for _, c := range p.Constrains {
		if err := clause.WellFormed(c.Guard); err != nil {
			return fmt.Errorf("constrain %s: %w", c.Event.Channel, err)
		}
		if len(c.Guard.Conjuncts) == 0 {
			return fmt.Errorf("constrain %s has an empty guard, which constrains nothing", c.Event.Channel)
		}
		allowed := maps.Clone(mapNames)
		for _, fl := range c.Event.Fields {
			if fl.Send {
				return fmt.Errorf("constrain %s sends %s: every field of a global event is ?", c.Event.Channel, fl.Name)
			}
			allowed[fl.Name] = true
		}
		for _, v := range clause.FreeVars(c.Guard) {
			if v.Primed {
				return fmt.Errorf("constrain %s reads %s', a primed name: it reads the maps as they are", c.Event.Channel, v.Name)
			}
			if !allowed[v.Name] {
				return fmt.Errorf("constrain %s reads %s, which is no map and no field of its event", c.Event.Channel, v.Name)
			}
		}
		matched := 0
		for i, e := range edges {
			if e.Event.Channel != c.Event.Channel {
				continue
			}
			if !e.Event.SameShape(c.Event) {
				return fmt.Errorf("constrain %s: a generated edge on %s has fields of another shape, which it would leave unconstrained", c.Event.Channel, c.Event.Channel)
			}
			matched++
			edges[i].Guard = Conj(e.Guard, renamedTo(c, e.Event))
		}
		if matched == 0 {
			return fmt.Errorf("constrain %s: no generated edge is on %s with fields of those directions", c.Event.Channel, c.Event.Channel)
		}
	}
	return nil
}

// Warning is something Promote dropped that the author wrote: Map and
// Event say where, Text what (docs/PROMOTION.md, the warnings table).
type Warning struct {
	Map   string
	Event string
	Text  string
}

// dropped warns of the posts of f that Promote drops: the post of its
// local start edge and of a deletion say nothing, there being no instance
// for them to be about.
func dropped(f Family) []Warning {
	var ws []Warning
	if len(f.Local.InitPost.Conjuncts) > 0 {
		ws = append(ws, Warning{Map: f.Map, Event: "[*]", Text: "the post of the start edge says nothing: the start state means that the instance does not exist"})
	}
	for _, e := range f.Local.Edges {
		if e.Dst == f.Local.Init && len(e.Post.Conjuncts) > 0 {
			ws = append(ws, Warning{Map: f.Map, Event: e.Event.Channel, Text: "the post of a deletion says nothing: the instance no longer exists"})
		}
	}
	return ws
}

// lifecycle reports, as an error, a local diagram that breaks the
// local-diagram contract (docs/PROMOTION.md): its start state means that
// the instance does not exist.
func lifecycle(f Family) error {
	s0 := f.Local.Init
	if len(f.Local.Vars[s0]) > 0 {
		return fmt.Errorf("the start state %s holds no state variables: it means that the instance does not exist", s0)
	}
	if len(f.Local.Ends) > 0 {
		return fmt.Errorf("a local diagram has no end edge: an edge into the start state %s deletes the instance", s0)
	}
	for _, e := range f.Local.Edges {
		if e.Src == s0 && e.Dst == s0 {
			return fmt.Errorf("%s is a self-loop on the start state %s, an event of an instance that is not there", e.Event.Channel, s0)
		}
	}
	return nil
}

// moveOf is the move of the instance bound to id taking e: a creation when
// e leaves f's start state, a deletion when it enters it.
func moveOf(f Family, id string, e Edge) move {
	k := local
	switch {
	case e.Src == f.Local.Init:
		k = creation
	case e.Dst == f.Local.Init:
		k = deletion
	}
	return move{f: f, id: id, e: e, kind: k}
}

// kind says what a move does to its instance.
type kind int

const (
	// local takes a local edge of an instance that exists.
	local kind = iota
	// creation creates an instance that does not exist, taking an edge out
	// of the local start state.
	creation
	// deletion deletes an instance, taking an edge into the local start
	// state.
	deletion
)

// move is one instance's part in a step: the edge e of the family f, taken
// by the instance bound to id, as kind says.
type move struct {
	f    Family
	id   string
	e    Edge
	kind kind
}

func promoteAlone(p Promotion, m move, mode string) (Edge, error) {
	f, e := m.f, m.e
	if slices.ContainsFunc(e.Event.Fields, func(fl Field) bool { return fl.Name == f.ID }) {
		return Edge{}, fmt.Errorf("%s has a field named like the ID %s", e.Event.Channel, f.ID)
	}
	guard, post, err := step(p, []move{m}, map[string]bool{f.ID: true})
	if err != nil {
		return Edge{}, err
	}
	if e.Event.Channel == Tau {
		// An internal event carries no fields (docs/SYNTAX.md); hiding binds
		// a received one by ∃ instead (Hide), so a local τ edge with fields
		// is not one a diagram can have.
		if len(e.Event.Fields) > 0 {
			return Edge{}, fmt.Errorf("a tau edge from %s has fields, which an internal event cannot carry", e.Src)
		}
		// An internal event has no argument: the instance is any that can
		// take it (docs/PROMOTION.md). The post binds its own instance, one
		// the guard holds of: ∃id. (g ∧ p), as Hide does.
		g, err := Bind(guard, f.ID, f.ID)
		if err != nil {
			return Edge{}, err
		}
		q, err := Bind(Conj(guard, post), f.ID, f.ID)
		if err != nil {
			return Edge{}, err
		}
		return Edge{Src: mode, Dst: mode, Event: Event{Channel: Tau}, Guard: g, Post: q}, nil
	}
	fields, pins, err := carried(m)
	if err != nil {
		return Edge{}, err
	}
	fields = append([]Field{{Name: f.ID}}, fields...)
	return Edge{Src: mode, Dst: mode, Event: Event{Channel: e.Event.Channel, Fields: fields}, Guard: Conj(pins, guard), Post: post}, nil
}

// carried are the fields of m's local event on the global event, and the
// guard that fixes those it sends. The global diagram holds maps, not the
// instance's variables, so every field is received there; a field m sends
// of its instance's variable v is fixed to m[x].v (@eq).
func carried(m move) ([]Field, clause.And, error) {
	vars := map[string]bool{}
	for _, v := range m.f.Local.Vars[m.e.Src] {
		vars[v] = true
	}
	received := map[string]bool{}
	for _, fl := range m.e.Event.Fields {
		if !fl.Send {
			received[fl.Name] = true
		}
	}
	var fields []Field
	var pins []clause.Formula
	for _, fl := range m.e.Event.Fields {
		fields = append(fields, Field{Name: fl.Name})
		switch {
		case fl.Send && !vars[fl.Name] && !received[fl.Name]:
			// As in the local diagram's own expansion: a send names a value
			// the step has bound.
			return nil, clause.And{}, fmt.Errorf("%s sends %s, which is bound neither by its instance nor by the event", m.e.Event.Channel, fl.Name)
		case fl.Send && vars[fl.Name]:
			instance := clause.Index{Map: clause.Free{Name: m.f.Map}, Key: clause.Free{Name: m.id}}
			pins = append(pins, clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: fl.Name}, clause.Field{Of: instance, Name: fl.Name}}})
		case !fl.Send && vars[fl.Name]:
			return nil, clause.And{}, fmt.Errorf("%s receives %s, which its instance holds", m.e.Event.Channel, fl.Name)
		}
	}
	return fields, clause.And{Conjuncts: pins}, nil
}

func promoteSync(p Promotion, s Sync, mode string) ([]Edge, error) {
	if s.Event == Tau {
		return nil, fmt.Errorf("tau is internal and cannot be synchronised")
	}
	ids := map[string]bool{}
	for i, a := range s.Targets {
		if a.Event == "" {
			return nil, fmt.Errorf("%s(%s) names no local event", a.Map, a.ID)
		}
		if a.Event == Tau {
			return nil, fmt.Errorf("%s(%s) takes tau, which is internal and cannot be synchronised", a.Map, a.ID)
		}
		ids[a.ID] = true
		for _, b := range s.Targets[:i] {
			if a.Map == b.Map && a.ID == b.ID {
				return nil, fmt.Errorf("%s(%s) is named twice: one instance plays one role", a.Map, a.ID)
			}
			if a.Map == b.Map && a.Event == b.Event {
				return nil, fmt.Errorf("%s takes its local event %s twice", a.Map, a.Event)
			}
		}
	}
	if err := readsBound(p, s); err != nil {
		return nil, err
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
			if e.Event.Channel != tg.Event {
				continue
			}
			for _, c := range combos {
				grown = append(grown, append(slices.Clone(c), moveOf(f, tg.ID, e)))
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
		// The fields are merged by name (docs/CLAUSE_DESIGN.md): one name
		// is one field, which every move naming it agrees on.
		merged := slices.Clone(fields)
		var pins []clause.And
		for _, m := range c {
			fs, pin, err := carried(m)
			if err != nil {
				return nil, err
			}
			pins = append(pins, pin)
			for _, fl := range fs {
				if ids[fl.Name] {
					return nil, fmt.Errorf("%s has a field named like the ID %s", m.e.Event.Channel, fl.Name)
				}
				if !slices.Contains(merged, fl) {
					merged = append(merged, fl)
				}
			}
		}
		guard, post, err := step(p, c, ids)
		if err != nil {
			return nil, err
		}
		out = append(out, Edge{
			Src: mode, Dst: mode,
			Event: Event{Channel: s.Event, Fields: merged},
			Guard: Conj(append([]clause.And{{Conjuncts: distinct}, s.Guard, guard}, pins...)...),
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
			posts = append(posts, Unchanged([]string{f.Map}))
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
			g, q, err := moved(f, m, before, after)
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

// moved is the guard and post of one move, with the instance's local
// variables substituted: v is m[x].v in the map before the step (the moves
// of one step are of different instances, so every intermediate map agrees
// with m at x), v' is after[x].v.
func moved(f Family, m move, before, after clause.Term) (clause.And, clause.And, error) {
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
	facts := func(fs ...clause.Formula) clause.And { return clause.And{Conjuncts: fs} }
	frame := clause.Builtin{Op: clause.OpFrame, Args: []clause.Term{after, before, x}}
	at := func(m clause.Term, state string) clause.Formula {
		return clause.Builtin{Op: clause.OpAt, Args: []clause.Term{clause.Index{Map: m, Key: x}}, State: state}
	}
	switch m.kind {
	case creation:
		return Conj(facts(clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{x, now}}), sub(m.e.Guard)),
			Conj(facts(frame, at(after, m.e.Dst)), sub(m.e.Post)), nil
	case deletion:
		return Conj(facts(at(now, m.e.Src)), sub(m.e.Guard)),
			facts(frame, clause.Builtin{Op: clause.OpNotIn, Args: []clause.Term{x, after}}), nil
	}
	return Conj(facts(at(now, m.e.Src)), sub(m.e.Guard)),
		Conj(facts(frame, at(after, m.e.Dst)), sub(m.e.Post)), nil
}

// MapType is the type of the map of f over the IDs ids: every partial map
// from them to an instance at a state of f's local diagram other than its
// start state, which is absence, its variables at values of their types in
// env.
func MapType(f Family, ids []Value, env Env) ([]Value, error) {
	var recs []Value
	for _, state := range f.Local.States {
		if state == f.Local.Init {
			continue
		}
		fields := []map[string]Value{{}}
		for _, v := range f.Local.Vars[state] {
			vals, err := env.values(v)
			if err != nil {
				return nil, fmt.Errorf("core.MapType: %s: %w", f.Map, err)
			}
			var grown []map[string]Value
			for _, fs := range fields {
				for _, val := range vals {
					g := map[string]Value{v: val}
					for k, w := range fs {
						g[k] = w
					}
					grown = append(grown, g)
				}
			}
			fields = grown
		}
		for _, fs := range fields {
			recs = append(recs, Rec{State: state, Fields: fs})
		}
	}
	maps := []Map{{}}
	for _, id := range ids {
		var grown []Map
		for _, m := range maps {
			absent := Map{}
			for k, v := range m {
				absent[k] = v
			}
			grown = append(grown, absent)
			for _, r := range recs {
				g := Map{Key(id): r}
				for k, v := range m {
					g[k] = v
				}
				grown = append(grown, g)
			}
		}
		maps = grown
	}
	out := make([]Value, len(maps))
	for i, m := range maps {
		out[i] = m
	}
	return out, nil
}

// readsBound reports, as an error, a clause of the sync s that reads what s
// does not give it: a name that is no map, no target's ID and no field of a
// target's local event, or an entry of a map keyed by anything but the ID
// of one of that map's targets (docs/CLAUSE_DESIGN.md, Control state).
func readsBound(p Promotion, s Sync) error {
	allowed := map[string]bool{}
	keys := map[string]map[string]bool{}
	for _, f := range p.Families {
		allowed[f.Map] = true
		keys[f.Map] = map[string]bool{}
	}
	for _, tg := range s.Targets {
		allowed[tg.ID] = true
		if keys[tg.Map] != nil {
			keys[tg.Map][tg.ID] = true
		}
		if f, err := familyOf(p, tg.Map); err == nil {
			for _, e := range f.Local.Edges {
				if e.Event.Channel == tg.Event {
					for _, fl := range e.Event.Fields {
						allowed[fl.Name] = true
					}
				}
			}
		}
	}
	var err error
	var walk func(t clause.Term)
	walk = func(t clause.Term) {
		switch t := t.(type) {
		case clause.Index:
			m, okm := t.Map.(clause.Free)
			k, okk := t.Key.(clause.Free)
			if okm && keys[m.Name] != nil && (!okk || !keys[m.Name][k.Name]) && err == nil {
				err = fmt.Errorf("its clause reads %s[%s], an instance it does not bind", m.Name, keyName(t.Key))
			}
			walk(t.Map)
			walk(t.Key)
		case clause.Field:
			walk(t.Of)
		case clause.App:
			for _, a := range t.Args {
				walk(a)
			}
		}
	}
	clause.EachTerm(s.Guard, walk)
	if err != nil {
		return err
	}
	for _, v := range clause.FreeVars(s.Guard) {
		if !allowed[v.Name] {
			return fmt.Errorf("its clause reads %s, which is no map, no target and no field", v.Name)
		}
	}
	return nil
}

func keyName(t clause.Term) string {
	if f, ok := t.(clause.Free); ok {
		return f.Name
	}
	return fmt.Sprintf("%v", t)
}
