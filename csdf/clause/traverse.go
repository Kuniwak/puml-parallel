package clause

// MapTerm rebuilds t bottom-up, replacing each variable (Free or Bound) by
// what leaf gives for it; a literal is no variable and stays. depth is the number of binders t sits under, and
// leaf is told it, so that it can tell a Bound that points outside from one
// that does not.
func MapTerm(t Term, depth int, leaf func(v Term, depth int) Term) Term {
	switch t := t.(type) {
	case Free, Bound:
		return leaf(t, depth)
	case Lit:
		return t
	case Index:
		return Index{Map: MapTerm(t.Map, depth, leaf), Key: MapTerm(t.Key, depth, leaf)}
	case Field:
		return Field{Of: MapTerm(t.Of, depth, leaf), Name: t.Name}
	case App:
		return App{Fn: t.Fn, Args: mapTermList(t.Args, depth, leaf)}
	}
	panic("clause.MapTerm: unknown term")
}

func mapTermList(ts []Term, depth int, leaf func(v Term, depth int) Term) []Term {
	out := make([]Term, len(ts))
	for i, t := range ts {
		out[i] = MapTerm(t, depth, leaf)
	}
	return out
}

// MapFormula rebuilds f, replacing each variable of each of its terms
// through leaf as MapTerm does; depth counts the binders of f a term sits
// under, starting from depth.
func MapFormula(f Formula, depth int, leaf func(v Term, depth int) Term) Formula {
	switch f := f.(type) {
	case And:
		return mapAnd(f, depth, leaf)
	case Exists:
		return Exists{Hint: f.Hint, Type: f.Type, Body: mapAnd(f.Body, depth+1, leaf)}
	case Atom:
		parts := make([]Part, len(f.Parts))
		for i, p := range f.Parts {
			if h, ok := p.(Hole); ok {
				parts[i] = Hole{Term: MapTerm(h.Term, depth, leaf)}
			} else {
				parts[i] = p
			}
		}
		return Atom{Parts: parts}
	case Builtin:
		return Builtin{Op: f.Op, Args: mapTermList(f.Args, depth, leaf), State: f.State}
	}
	panic("clause.MapFormula: unknown formula")
}

func mapAnd(f And, depth int, leaf func(v Term, depth int) Term) And {
	cs := make([]Formula, len(f.Conjuncts))
	for i, c := range f.Conjuncts {
		cs[i] = MapFormula(c, depth, leaf)
	}
	return And{Conjuncts: cs}
}

// EachVar calls visit on each variable of f, in order of appearance, with
// the number of binders it sits under.
func EachVar(f Formula, visit func(v Term, depth int)) {
	MapFormula(f, 0, func(v Term, depth int) Term { visit(v, depth); return v })
}

// Args are the terms the atom reads, its holes in order (docs/CLAUSE_DESIGN.md,
// P9); a term read twice is there twice.
func (a Atom) Args() []Term {
	var args []Term
	for _, p := range a.Parts {
		if h, ok := p.(Hole); ok {
			args = append(args, h.Term)
		}
	}
	return args
}

// FreeVars are the free variables of f in order of appearance, each once.
func FreeVars(f Formula) []Free {
	var vs []Free
	seen := map[Free]bool{}
	EachVar(f, func(v Term, _ int) {
		if fv, ok := v.(Free); ok && !seen[fv] {
			seen[fv] = true
			vs = append(vs, fv)
		}
	})
	return vs
}

// EachTerm calls visit on each term f's conjuncts read, outermost first: an
// atom's holes and a builtin's arguments, under any ∃.
func EachTerm(f Formula, visit func(Term)) {
	switch f := f.(type) {
	case And:
		for _, c := range f.Conjuncts {
			EachTerm(c, visit)
		}
	case Exists:
		EachTerm(f.Body, visit)
	case Atom:
		for _, a := range f.Args() {
			visit(a)
		}
	case Builtin:
		for _, a := range f.Args {
			visit(a)
		}
	}
}
