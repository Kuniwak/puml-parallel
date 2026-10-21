package clause

import "reflect"

// Normalize is f in normal form: conjunctions flattened into one And, the
// text of each atom merged and never empty, and no empty list where nil
// says the same. Print and AlphaEqual are defined on it: Print(f) is
// Print(Normalize(f)), and Parse(Print(f)) is α-equivalent to f.
func Normalize(f Formula) And {
	return And{Conjuncts: normalConjuncts(f, nil)}
}

func normalConjuncts(f Formula, out []Formula) []Formula {
	switch f := f.(type) {
	case And:
		for _, c := range f.Conjuncts {
			out = normalConjuncts(c, out)
		}
		return out
	case Exists:
		return append(out, Exists{Hint: f.Hint, Body: Normalize(f.Body)})
	case Atom:
		return append(out, normalAtom(f))
	case Builtin:
		return append(out, Builtin{Op: f.Op, Args: normalTerms(f.Args), State: f.State})
	}
	panic("clause.Normalize: unknown formula")
}

func normalAtom(a Atom) Atom {
	var parts []Part
	for _, p := range a.Parts {
		switch p := p.(type) {
		case Text:
			if p == "" {
				continue
			}
			if n := len(parts); n > 0 {
				if prev, ok := parts[n-1].(Text); ok {
					parts[n-1] = prev + p
					continue
				}
			}
			parts = append(parts, p)
		case Hole:
			parts = append(parts, Hole{Term: normalTerm(p.Term)})
		}
	}
	return Atom{Parts: parts}
}

func normalTerm(t Term) Term {
	switch t := t.(type) {
	case Index:
		return Index{Map: normalTerm(t.Map), Key: normalTerm(t.Key)}
	case Field:
		return Field{Of: normalTerm(t.Of), Name: t.Name}
	case App:
		return App{Fn: t.Fn, Args: normalTerms(t.Args)}
	}
	return t
}

func normalTerms(ts []Term) []Term {
	if len(ts) == 0 {
		return nil
	}
	out := make([]Term, len(ts))
	for i, t := range ts {
		out[i] = normalTerm(t)
	}
	return out
}

// AlphaEqual reports whether a and b are the same clause: equal in normal
// form, whatever the hints of their binders.
func AlphaEqual(a, b Formula) bool {
	return reflect.DeepEqual(withoutHints(Normalize(a)), withoutHints(Normalize(b)))
}

func withoutHints(f And) And {
	cs := make([]Formula, len(f.Conjuncts))
	for i, c := range f.Conjuncts {
		if e, ok := c.(Exists); ok {
			c = Exists{Body: withoutHints(e.Body)}
		}
		cs[i] = c
	}
	return And{Conjuncts: cs}
}
