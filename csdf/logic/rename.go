package logic

// Rename is f with each free variable that m maps renamed to its image. A
// variable a quantifier binds is not renamed under it. The images are not
// checked against the variables f binds: a caller renames to names no
// quantifier of f binds, or a renamed variable is captured.
func Rename(f Formula, m map[Var]Var) Formula {
	switch f := f.(type) {
	case constant:
		return f
	case atom:
		return atom{name: f.name, quoted: f.quoted, args: renameTerms(f.args, m)}
	case equality:
		return equality{left: renameTerm(f.left, m), right: renameTerm(f.right, m)}
	case not:
		return not{operand: Rename(f.operand, m)}
	case junction:
		ops := make([]Formula, len(f.operands))
		for i, o := range f.operands {
			ops[i] = Rename(o, m)
		}
		return junction{and: f.and, operands: ops}
	case implies:
		return implies{antecedent: Rename(f.antecedent, m), consequent: Rename(f.consequent, m)}
	case quantifier:
		inner := make(map[Var]Var, len(m))
		for k, v := range m {
			inner[k] = v
		}
		for _, v := range f.vars {
			delete(inner, v)
		}
		return quantifier{forall: f.forall, vars: f.vars, types: f.types, body: Rename(f.body, inner)}
	}
	panic("logic.Rename: no formula")
}

func renameTerms(ts []Term, m map[Var]Var) []Term {
	out := make([]Term, len(ts))
	for i, t := range ts {
		out[i] = renameTerm(t, m)
	}
	return out
}

func renameTerm(t Term, m map[Var]Var) Term {
	switch t := t.(type) {
	case Var:
		if v, ok := m[t]; ok {
			return v
		}
		return t
	case index:
		return index{m: renameTerm(t.m, m), key: renameTerm(t.key, m)}
	case field:
		return field{of: renameTerm(t.of, m), name: t.name}
	case apply:
		return apply{fn: t.fn, args: renameTerms(t.args, m)}
	}
	panic("logic.Rename: no term")
}
