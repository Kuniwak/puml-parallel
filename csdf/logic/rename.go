package logic

import (
	"fmt"
	"slices"
)

// Rename is f with each free variable that m maps renamed to its image. A
// variable a quantifier binds is not renamed under it, and a quantifier that
// binds a name an image takes in its body is renamed apart first, to a fresh
// name, so that no renamed variable is captured.
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
		// A bound variable an image takes would capture it, so it is renamed
		// to a name free in the renamed body nowhere.
		images, taken := map[Var]bool{}, map[Var]bool{}
		for _, v := range FreeVars(f.body) {
			if w, ok := inner[v]; ok {
				images[w], taken[w] = true, true
			} else {
				taken[v] = true
			}
		}
		vars := slices.Clone(f.vars)
		for i, v := range f.vars {
			if images[v] {
				fresh := freshVar(v, taken, f.vars)
				taken[fresh] = true
				inner[v] = fresh
				vars[i] = fresh
			}
		}
		return quantifier{forall: f.forall, vars: vars, types: f.types, body: Rename(f.body, inner)}
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

// freshVar is v with the least number after it that names nothing taken
// and none of the variables bound beside it.
func freshVar(v Var, taken map[Var]bool, bound []Var) Var {
	for i := 1; ; i++ {
		w := Var(fmt.Sprintf("%s%d", v, i))
		if !taken[w] && !slices.Contains(bound, w) {
			return w
		}
	}
}
