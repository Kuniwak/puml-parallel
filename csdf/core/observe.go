package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
)

// observed is p with every clause of its global diagram read through the
// observations of the local diagrams (docs/PROMOTION.md, Observations): an
// @at naming a view names its states instead. It reports, as an error, a
// clause that reads of an instance of a family whose local diagram
// declares observations what that diagram does not declare: an @at naming
// a state rather than a view, a variable not declared, or the instance as a
// whole. Whether an instance exists is always read.
func observed(p Promotion) (Promotion, error) {
	obs := map[string]*Observation{}
	for _, f := range p.Families {
		if f.Local != nil && f.Local.Observes != nil {
			obs[f.Map] = f.Local.Observes
		}
	}
	if len(obs) == 0 {
		return p, nil
	}
	read := func(where string, c clause.And) (clause.And, error) {
		out, err := throughObservations(obs, c)
		if err != nil {
			return clause.And{}, fmt.Errorf("%s: %w", where, err)
		}
		return out, nil
	}
	var err error
	if p.InitPost, err = read("the global start edge", p.InitPost); err != nil {
		return Promotion{}, err
	}
	p.Syncs = slices.Clone(p.Syncs)
	for i, s := range p.Syncs {
		if p.Syncs[i].Guard, err = read("sync "+s.Event, s.Guard); err != nil {
			return Promotion{}, err
		}
	}
	p.Constrains = slices.Clone(p.Constrains)
	for i, c := range p.Constrains {
		if p.Constrains[i].Guard, err = read("constrain "+c.Event.Channel, c.Guard); err != nil {
			return Promotion{}, err
		}
	}
	p.Edges = slices.Clone(p.Edges)
	for i, e := range p.Edges {
		name := fmt.Sprintf("%s --> %s : %s", e.Src, e.Dst, e.Event.Channel)
		if p.Edges[i].Guard, err = read(name, e.Guard); err != nil {
			return Promotion{}, err
		}
		if p.Edges[i].Post, err = read(name, e.Post); err != nil {
			return Promotion{}, err
		}
	}
	return p, nil
}

// throughObservations is c with each @at of an observed family's instance
// naming the states of the views it names, obs being the observation of
// each observed family by its map.
func throughObservations(obs map[string]*Observation, c clause.And) (clause.And, error) {
	out := clause.And{Conjuncts: make([]clause.Formula, len(c.Conjuncts))}
	for i, f := range c.Conjuncts {
		g, err := formulaThroughObservations(obs, f)
		if err != nil {
			return clause.And{}, err
		}
		out.Conjuncts[i] = g
	}
	return out, nil
}

func formulaThroughObservations(obs map[string]*Observation, f clause.Formula) (clause.Formula, error) {
	switch f := f.(type) {
	case clause.And:
		return throughObservations(obs, f)
	case clause.Exists:
		body, err := throughObservations(obs, f.Body)
		if err != nil {
			return nil, err
		}
		f.Body = body
		return f, nil
	case clause.Builtin:
		if m, o := observedAt(obs, f); o != nil {
			states, err := viewStates(m, o, f.States)
			if err != nil {
				return nil, err
			}
			f.States = states
			return f, nil
		}
		for _, a := range f.Args {
			if err := termObserved(obs, a); err != nil {
				return nil, err
			}
		}
		return f, nil
	case clause.Atom:
		for _, a := range f.Args() {
			if err := termObserved(obs, a); err != nil {
				return nil, err
			}
		}
		return f, nil
	}
	return f, nil
}

// observedAt is, for an @at of an instance of an observed family, the map
// of the family and its observation; o is nil for any other formula.
func observedAt(obs map[string]*Observation, b clause.Builtin) (m clause.Free, o *Observation) {
	if b.Op != clause.OpAt || len(b.Args) == 0 {
		return clause.Free{}, nil
	}
	ix, ok := b.Args[0].(clause.Index)
	if !ok {
		return clause.Free{}, nil
	}
	m, ok = ix.Map.(clause.Free)
	if !ok {
		return clause.Free{}, nil
	}
	return m, obs[m.Name]
}

// viewStates are the states of the views names, each once, in order; a
// name that is no view of o is an error.
func viewStates(m clause.Free, o *Observation, names []string) ([]string, error) {
	var out []string
	for _, n := range names {
		states, ok := o.Views[n]
		if !ok {
			return nil, fmt.Errorf("@at(%s[…], %s) reads %s, which the local diagram of %s does not declare as a view; it reads only its views (%s); add ' CSDF-OBSERVE <view> = %s to it", termName(m), strings.Join(names, ", "), n, m.Name, viewNames(o), n)
		}
		for _, s := range states {
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// termObserved reports, as an error, a part of t that reads of an
// instance of an observed family a variable its local diagram does not
// declare, or the instance as a whole.
func termObserved(obs map[string]*Observation, t clause.Term) error {
	switch t := t.(type) {
	case clause.Field:
		if ix, ok := t.Of.(clause.Index); ok {
			if m, o := observedMap(obs, ix); o != nil {
				if !slices.Contains(o.Vars, t.Name) {
					return fmt.Errorf("it reads %s[%s].%s, but the local diagram of %s does not declare %s observed; add ' CSDF-OBSERVE %s to it", termName(m), entryKey(ix.Key), t.Name, m.Name, t.Name, t.Name)
				}
				return termObserved(obs, ix.Key)
			}
		}
		return termObserved(obs, t.Of)
	case clause.Index:
		if m, o := observedMap(obs, t); o != nil {
			return fmt.Errorf("it reads %s[%s], an instance as a whole, but the local diagram of %s declares what it observes; read a declared variable, or @at a declared view", termName(m), entryKey(t.Key), m.Name)
		}
		if err := termObserved(obs, t.Map); err != nil {
			return err
		}
		return termObserved(obs, t.Key)
	case clause.App:
		for _, a := range t.Args {
			if err := termObserved(obs, a); err != nil {
				return err
			}
		}
	}
	return nil
}

// observedMap is, for an entry of the map of an observed family, the map
// and its observation; o is nil for any other entry.
func observedMap(obs map[string]*Observation, ix clause.Index) (clause.Free, *Observation) {
	m, ok := ix.Map.(clause.Free)
	if !ok {
		return clause.Free{}, nil
	}
	return m, obs[m.Name]
}

func termName(m clause.Free) string {
	if m.Primed {
		return m.Name + "'"
	}
	return m.Name
}

func viewNames(o *Observation) string {
	names := make([]string, 0, len(o.Views))
	for n := range o.Views {
		names = append(names, n)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// entryKey is how a message writes the key of an entry: the name of a free
// variable, … for anything else.
func entryKey(k clause.Term) string {
	if f, ok := k.(clause.Free); ok {
		return f.Name
	}
	return "…"
}
