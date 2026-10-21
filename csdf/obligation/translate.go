// Package obligation states what a diagram whose clauses are csdf/clause
// formulas has to satisfy, as formulas of csdf/logic (docs/CLAUSE_DESIGN.md,
// O8). An atom is an uninterpreted predicate, named by its text without its
// holes and applied to its holes (P15, P9); a builtin is an equality, or a
// predicate symbol whose meaning a target defines once (see the Pred*
// constants); an ∃ a tool wrote is an existential of its type.
package obligation

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// The predicate symbols the map builtins become. A target defines each once:
//
//	csdf_in(k, m)             k is in the domain of m
//	csdf_empty(m)             m has no entries
//	csdf_update(m', m, k, v)  m' is m with k mapped to v
//	csdf_frame(m', m, k…)     m' and m agree everywhere but at the keys
//	csdf_at(t, S)             the instance t is at the state S
//
// @notin is the negation of csdf_in; @eq and @ne are equalities.
const (
	PredIn     = "csdf_in"
	PredEmpty  = "csdf_empty"
	PredUpdate = "csdf_update"
	PredFrame  = "csdf_frame"
	PredAt     = "csdf_at"
)

// Formula is the formula of the logic that f states. A free variable is the
// variable of its name, primed with ' when it is a value after the step; a
// variable an ∃ binds takes the name of its hint, made fresh against every
// other name the formula reads.
func Formula(f clause.Formula) (logic.Formula, error) {
	if err := clause.WellFormed(f); err != nil {
		return nil, fmt.Errorf("obligation.Formula: %w", err)
	}
	avoid := map[string]bool{}
	for _, v := range clause.FreeVars(f) {
		avoid[v.Name] = true
	}
	return translate(f, avoid)
}

func translate(f clause.Formula, avoid map[string]bool) (logic.Formula, error) {
	switch f := f.(type) {
	case clause.And:
		fs := make([]logic.Formula, 0, len(f.Conjuncts))
		for _, c := range f.Conjuncts {
			g, err := translate(c, avoid)
			if err != nil {
				return nil, err
			}
			fs = append(fs, g)
		}
		return logic.And(fs...), nil
	case clause.Atom:
		fn, err := clause.FunctionOf(f)
		if err != nil {
			return nil, err
		}
		ts, err := terms(fn.Args)
		if err != nil {
			return nil, err
		}
		return logic.Pred(fn.Name, true, ts...), nil
	case clause.Builtin:
		return builtin(f)
	case clause.Exists:
		name := clause.Fresh(avoid, f.Hint)
		body, err := clause.Open(f, clause.Free{Name: name})
		if err != nil {
			return nil, err
		}
		inner := make(map[string]bool, len(avoid)+1)
		for n := range avoid {
			inner[n] = true
		}
		inner[name] = true
		g, err := translate(body, inner)
		if err != nil {
			return nil, err
		}
		return logic.ExistsTyped([]logic.Var{logic.Var(name)}, []string{f.Type}, g), nil
	}
	return nil, fmt.Errorf("obligation.Formula: %#v is no formula", f)
}

func builtin(b clause.Builtin) (logic.Formula, error) {
	args, err := terms(b.Args)
	if err != nil {
		return nil, err
	}
	switch b.Op {
	case clause.OpEq:
		return logic.Eq(args[0], args[1]), nil
	case clause.OpNe:
		return logic.Not(logic.Eq(args[0], args[1])), nil
	case clause.OpIn:
		return logic.Pred(PredIn, false, args...), nil
	case clause.OpNotIn:
		return logic.Not(logic.Pred(PredIn, false, args...)), nil
	case clause.OpEmpty:
		return logic.Pred(PredEmpty, false, args...), nil
	case clause.OpUpdate:
		return logic.Pred(PredUpdate, false, args...), nil
	case clause.OpFrame:
		return logic.Pred(PredFrame, false, args...), nil
	case clause.OpAt:
		return logic.Pred(PredAt, false, args[0], logic.Apply(b.State)), nil
	}
	return nil, fmt.Errorf("obligation.Formula: no builtin @%s", b.Op)
}

func terms(ts []clause.Term) ([]logic.Term, error) {
	out := make([]logic.Term, len(ts))
	for i, t := range ts {
		u, err := term(t)
		if err != nil {
			return nil, err
		}
		out[i] = u
	}
	return out, nil
}

// Var is the variable of the logic a free variable of a clause is.
func Var(v clause.Free) logic.Var {
	if v.Primed {
		return logic.Var(v.Name + "'")
	}
	return logic.Var(v.Name)
}

func term(t clause.Term) (logic.Term, error) {
	switch t := t.(type) {
	case clause.Free:
		return Var(t), nil
	case clause.Index:
		m, err := term(t.Map)
		if err != nil {
			return nil, err
		}
		k, err := term(t.Key)
		if err != nil {
			return nil, err
		}
		return logic.Index(m, k), nil
	case clause.Field:
		of, err := term(t.Of)
		if err != nil {
			return nil, err
		}
		return logic.Field(of, t.Name), nil
	case clause.App:
		args, err := terms(t.Args)
		if err != nil {
			return nil, err
		}
		return logic.Apply(t.Fn, args...), nil
	}
	return nil, fmt.Errorf("obligation.Formula: %#v reads an index no ∃ binds", t)
}
