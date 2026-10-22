package clause

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// WellFormed reports, as an error, what keeps f from being a clause the
// surface syntax can write: a name that is no identifier, an empty atom or
// text, an ∃ with no body, a builtin outside its table, or an index no
// binder binds. Print prints only well-formed formulas.
func WellFormed(f Formula) error {
	if err := wellFormed(f); err != nil {
		return fmt.Errorf("clause.WellFormed: %w", err)
	}
	return LocallyClosed(f)
}

func wellFormed(f Formula) error {
	switch f := f.(type) {
	case And:
		for _, c := range f.Conjuncts {
			if err := wellFormed(c); err != nil {
				return err
			}
		}
		return nil
	case Exists:
		if !ident.Is(f.Hint) {
			return fmt.Errorf("the hint %q of an ∃ is no identifier", f.Hint)
		}
		if !ident.Is(f.Type) {
			return fmt.Errorf("the type %q of the ∃ %s is no identifier", f.Type, f.Hint)
		}
		if len(f.Body.Conjuncts) == 0 {
			return fmt.Errorf("the ∃ %s has an empty body", f.Hint)
		}
		return wellFormed(f.Body)
	case Atom:
		if len(f.Parts) == 0 {
			return fmt.Errorf("an empty atom")
		}
		for _, p := range f.Parts {
			switch p := p.(type) {
			case Text:
				if p == "" {
					return fmt.Errorf("empty text in an atom")
				}
			case Hole:
				if err := wellFormedTerm(p.Term); err != nil {
					return err
				}
			}
		}
		return nil
	case Builtin:
		return wellFormedBuiltin(f)
	}
	return fmt.Errorf("an unknown formula %#v", f)
}

func wellFormedBuiltin(b Builtin) error {
	spec, ok := b.Op.Spec()
	if !ok {
		return fmt.Errorf("an unknown builtin @%s", b.Op)
	}
	if len(b.Args) < spec.Terms || (!spec.Variadic && len(b.Args) > spec.Terms) {
		return fmt.Errorf("@%s takes %d terms, not %d", b.Op, spec.Terms, len(b.Args))
	}
	if spec.State != (b.State != "") {
		return fmt.Errorf("@%s: a state %q where the table says state = %v", b.Op, b.State, spec.State)
	}
	if spec.State && !ident.Is(b.State) {
		return fmt.Errorf("@%s: the state %q is no identifier", b.Op, b.State)
	}
	for _, t := range b.Args {
		if err := wellFormedTerm(t); err != nil {
			return err
		}
	}
	return nil
}

func wellFormedTerm(t Term) error {
	switch t.(type) {
	case Index, Field:
		switch Root(t).(type) {
		case Free, Bound:
		default:
			return fmt.Errorf("a path starts from a variable, not from %#v", Root(t))
		}
	}
	switch t := t.(type) {
	case Free:
		if !ident.Is(t.Name) {
			return fmt.Errorf("the variable %q is no identifier", t.Name)
		}
	case Bound:
		if t.Index < 0 {
			return fmt.Errorf("a negative index %d", t.Index)
		}
	case Index:
		if err := wellFormedTerm(t.Map); err != nil {
			return err
		}
		return wellFormedTerm(t.Key)
	case Field:
		if !ident.Is(t.Name) {
			return fmt.Errorf("the field %q is no identifier", t.Name)
		}
		return wellFormedTerm(t.Of)
	case App:
		if !ident.Is(t.Fn) {
			return fmt.Errorf("the function %q is no identifier", t.Fn)
		}
		for _, a := range t.Args {
			if err := wellFormedTerm(a); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("an unknown term %#v", t)
	}
	return nil
}

// shaped reports, as an error, a formula, part or term of f that is none of
// the cases this package defines, nil included. The traversals panic on one,
// so every operation that takes a formula from outside checks this first.
func shaped(f Formula) error {
	switch f := f.(type) {
	case And:
		for _, c := range f.Conjuncts {
			if err := shaped(c); err != nil {
				return err
			}
		}
		return nil
	case Exists:
		return shaped(f.Body)
	case Atom:
		for _, p := range f.Parts {
			switch p := p.(type) {
			case Text:
			case Hole:
				if err := shapedTerm(p.Term); err != nil {
					return err
				}
			default:
				return fmt.Errorf("an unknown part %#v", p)
			}
		}
		return nil
	case Builtin:
		for _, t := range f.Args {
			if err := shapedTerm(t); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("an unknown formula %#v", f)
}

func shapedTerm(t Term) error {
	switch t := t.(type) {
	case Free, Bound:
		return nil
	case Index:
		if err := shapedTerm(t.Map); err != nil {
			return err
		}
		return shapedTerm(t.Key)
	case Field:
		return shapedTerm(t.Of)
	case App:
		for _, a := range t.Args {
			if err := shapedTerm(a); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("an unknown term %#v", t)
}
