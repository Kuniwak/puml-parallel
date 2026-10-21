package clause

import "fmt"

// LocallyClosed reports, as an error, an index of f that no binder of f
// binds. Close, Open and Substitute take only locally closed formulas, and
// Print prints only them.
func LocallyClosed(f Formula) error {
	if err := shaped(f); err != nil {
		return fmt.Errorf("clause.LocallyClosed: %w", err)
	}
	var err error
	EachVar(f, func(v Term, depth int) {
		if b, ok := v.(Bound); ok && b.Index >= depth && err == nil {
			err = fmt.Errorf("clause.LocallyClosed: the index %d is under %d binders", b.Index, depth)
		}
	})
	return err
}

// TermLocallyClosed reports, as an error, an index of t; a term alone has
// no binder, so any index in it is unbound.
func TermLocallyClosed(t Term) error {
	return LocallyClosed(Atom{Parts: []Part{Hole{Term: t}}})
}

// Close binds v, ranging over the type typ, in f: ∃v : typ. f, with the occurrences of v before the step
// becoming the index of the new binder. Bound variables are never primed,
// so v must not be, and v' stays free: it is the diagram's v after the step.
func Close(f Formula, v Free, typ string) (Exists, error) {
	if v.Primed {
		return Exists{}, fmt.Errorf("clause.Close: %s' is the value after the step and cannot be bound", v.Name)
	}
	if err := LocallyClosed(f); err != nil {
		return Exists{}, fmt.Errorf("clause.Close: %w", err)
	}
	body := MapFormula(f, 0, func(t Term, depth int) Term {
		if t == Term(v) {
			return Bound{Index: depth}
		}
		return t
	})
	return Exists{Hint: v.Name, Type: typ, Body: Normalize(body)}, nil
}

// Open is the body of e with its variable replaced by t.
func Open(e Exists, t Term) (And, error) {
	if err := LocallyClosed(e); err != nil {
		return And{}, fmt.Errorf("clause.Open: %w", err)
	}
	if err := TermLocallyClosed(t); err != nil {
		return And{}, fmt.Errorf("clause.Open: the term: %w", err)
	}
	body := MapFormula(e.Body, 0, func(v Term, depth int) Term {
		if b, ok := v.(Bound); ok && b.Index == depth {
			return t
		}
		return v
	})
	return Normalize(body), nil
}

// Substitute replaces every occurrence of the free variable v in f, with
// its priming, by t. Bound variables are indices, so nothing is captured.
func Substitute(f Formula, v Free, t Term) (And, error) {
	if err := LocallyClosed(f); err != nil {
		return And{}, fmt.Errorf("clause.Substitute: %w", err)
	}
	if err := TermLocallyClosed(t); err != nil {
		return And{}, fmt.Errorf("clause.Substitute: the term: %w", err)
	}
	return Normalize(MapFormula(f, 0, func(u Term, _ int) Term {
		if u == Term(v) {
			return t
		}
		return u
	})), nil
}
