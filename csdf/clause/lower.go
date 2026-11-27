package clause

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/v8/csdf/types"
)

// FormulaOf is the formula the clause s says: each name resolved against the
// ∃ that bind it, each type read. It is Parse's reading of a clause, given
// its syntax tree.
func FormulaOf(s Syntax) (And, error) {
	f, err := formula(s, false)
	if err != nil {
		return And{}, fmt.Errorf("clause.FormulaOf: %w", err)
	}
	return f, nil
}

// AuthoredFormulaOf is FormulaOf of a clause an author wrote, refusing as
// ParseAuthored does.
func AuthoredFormulaOf(s Syntax) (And, error) {
	f, err := formula(s, true)
	if err != nil {
		return And{}, fmt.Errorf("clause.AuthoredFormulaOf: %w", err)
	}
	return f, nil
}

func formula(s Syntax, authored bool) (And, error) {
	l := &lowerer{authored: authored}
	return l.clause(s)
}

// lowerer turns a syntax tree into a formula.
type lowerer struct {
	scope []string // names bound by enclosing ∃, innermost last
	// authored refuses an ∃ shadowing an enclosing one, which an author
	// never means; a tool's binders are nameless and only hinted.
	authored bool
}

func (l *lowerer) clause(s Syntax) (And, error) {
	if len(s.Conjuncts) == 0 {
		return And{}, nil
	}
	cs := make([]Formula, 0, len(s.Conjuncts))
	for _, c := range s.Conjuncts {
		f, err := l.conjunct(c)
		if err != nil {
			return And{}, err
		}
		cs = append(cs, f)
	}
	return And{Conjuncts: cs}, nil
}

func (l *lowerer) conjunct(c SynConjunct) (Formula, error) {
	switch c := c.(type) {
	case *SynExists:
		return l.exists(c)
	case *SynBuiltin:
		return l.builtin(c)
	case *SynAtom:
		return l.atom(c)
	}
	return nil, fmt.Errorf("clause: unknown conjunct %#v", c)
}

// exists reads @exists(name : type) (clause); name is Bound{0} in the
// clause.
func (l *lowerer) exists(e *SynExists) (Formula, error) {
	if l.authored && slices.Contains(l.scope, e.Name) {
		return nil, fmt.Errorf("@exists at %d: %s is bound by an enclosing @exists; give it another name", e.Pos, e.Name)
	}
	t, err := types.Parse(e.Type)
	if err != nil {
		return nil, fmt.Errorf("@exists at %d: the type %s ranges over: %w", e.Pos, e.Name, err)
	}
	l.scope = append(l.scope, e.Name)
	body, err := l.clause(e.Body)
	l.scope = l.scope[:len(l.scope)-1]
	if err != nil {
		return nil, err
	}
	return Exists{Hint: e.Name, Type: t.String(), Body: body}, nil
}

func (l *lowerer) builtin(b *SynBuiltin) (Formula, error) {
	out := Builtin{Op: b.Op, Args: []Term{}, States: b.States}
	for _, a := range b.Args {
		t, err := l.term(a)
		if err != nil {
			return nil, fmt.Errorf("@%s at %d: %w", b.Op, b.Pos, err)
		}
		out.Args = append(out.Args, t)
	}
	return out, nil
}

func (l *lowerer) atom(a *SynAtom) (Formula, error) {
	parts := make([]Part, 0, len(a.Parts))
	for _, p := range a.Parts {
		switch p := p.(type) {
		case *SynText:
			parts = append(parts, Text(p.Text))
		case *SynHole:
			t, err := l.hole(p)
			if err != nil {
				return nil, err
			}
			parts = append(parts, Hole{Term: t})
		}
	}
	return Atom{Parts: parts}, nil
}

// hole resolves the term of h; $name' names a variable after the step.
func (l *lowerer) hole(h *SynHole) (Term, error) {
	if v, ok := h.Term.(*SynVar); ok && !h.Braced {
		return l.variable(v.Name, v.Primed)
	}
	return l.term(h.Term)
}

// variable resolves name against the enclosing binders.
func (l *lowerer) variable(name string, primed bool) (Term, error) {
	for i := len(l.scope) - 1; i >= 0; i-- {
		if l.scope[i] == name {
			if primed {
				return nil, fmt.Errorf("%s is bound by @exists and has no value after the step", name)
			}
			return Bound{Index: len(l.scope) - 1 - i}, nil
		}
	}
	return Free{Name: name, Primed: primed}, nil
}

// term resolves t; a prime on the variable a path starts from, or after the
// path, primes that variable.
func (l *lowerer) term(t SynTerm) (Term, error) {
	switch t := t.(type) {
	case *SynLit:
		return Lit{Text: t.Text}, nil
	case *SynApp:
		args := []Term{}
		for _, a := range t.Args {
			at, err := l.term(a)
			if err != nil {
				return nil, err
			}
			args = append(args, at)
		}
		return App{Fn: t.Fn, Args: args}, nil
	case *SynPrime:
		return l.primed(t.Of, t.Pos)
	}
	if v, start := rootVar(t); v != nil && v.Primed {
		return l.primed(t, start)
	}
	return l.path(t)
}

// primed is the path t, the variable it starts from primed.
func (l *lowerer) primed(t SynTerm, start int) (Term, error) {
	u, err := l.path(t)
	if err != nil {
		return nil, err
	}
	p, err := PrimeRoot(u)
	if err != nil {
		return nil, fmt.Errorf("the term at %d starts from a variable bound by @exists, which has no value after the step", start)
	}
	return p, nil
}

// path resolves a variable and the indexes and fields after it, every prime
// left out.
func (l *lowerer) path(t SynTerm) (Term, error) {
	switch t := t.(type) {
	case *SynVar:
		return l.variable(t.Name, false)
	case *SynIndex:
		m, err := l.path(t.Map)
		if err != nil {
			return nil, err
		}
		k, err := l.term(t.Key)
		if err != nil {
			return nil, err
		}
		return Index{Map: m, Key: k}, nil
	case *SynField:
		o, err := l.path(t.Of)
		if err != nil {
			return nil, err
		}
		return Field{Of: o, Name: t.Name}, nil
	}
	return nil, fmt.Errorf("clause: unknown term %#v", t)
}

// rootVar is the variable the path t starts from, and where t starts; nil
// when t is no path.
func rootVar(t SynTerm) (*SynVar, int) {
	switch u := t.(type) {
	case *SynVar:
		return u, u.Pos
	case *SynIndex:
		v, _ := rootVar(u.Map)
		return v, u.Pos
	case *SynField:
		v, _ := rootVar(u.Of)
		return v, u.Pos
	}
	return nil, 0
}
