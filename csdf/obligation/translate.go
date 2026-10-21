// Package obligation states what a diagram whose clauses are csdf/clause
// formulas has to satisfy, as formulas of csdf/logic (docs/CLAUSE_DESIGN.md,
// O8). An atom is an uninterpreted predicate, named by its text without its
// holes and applied to its holes (P15, P9); a builtin is an equality, or a
// predicate symbol whose meaning Axioms states (see the Pred* constants); an
// ∃ a tool wrote is an existential of its type.
//
// Step is a relation, a hypothesis of the claims built on it. Enabled is
// that an edge can be taken: proven valid, it is sound without Axioms; used
// to show that some state can take the edge, it is sound only with them.
package obligation

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// The predicate symbols the map builtins become, which Axioms gives their
// meaning:
//
//	csdf_in(k, m)             k is in the domain of m
//	csdf_empty(m)             m has no entries
//	csdf_update(m', m, k, v)  m' is m with k mapped to v
//	csdf_frame(m', m, k…)     m' and m agree everywhere but at the keys
//	csdf_at(t, S)             the instance t is at the state S, written
//	                          as StateConstant names it
//
// @notin is the negation of csdf_in; @eq and @ne are equalities. A clause
// that reads an entry of a map is conjoined with csdf_in of its key, since a
// clause reading an entry that is not there is false.
const (
	PredIn     = "csdf_in"
	PredEmpty  = "csdf_empty"
	PredUpdate = "csdf_update"
	PredFrame  = "csdf_frame"
	PredAt     = "csdf_at"
)

// StatePrefix begins the constant a state of a local diagram becomes in csdf_at,
// so that a state never shares a name with a constant or an environment
// function of the clauses.
const StatePrefix = "csdf_state_"

// StateConstant is the constant the state of the builtin @at(t, S) becomes:
// StatePrefix, then the family the instance belongs to and a dot, then S. The
// family is the map the instance is an entry of, m for m[x], so that one state
// name in the local diagrams of two families is two constants; an instance
// that is no entry of a map names the state alone. The dot is in no name, so
// no two qualified states meet.
func StateConstant(b clause.Builtin) string {
	if family, ok := familyOf(b.Args[0]); ok {
		return StatePrefix + family + "." + b.State
	}
	return StatePrefix + b.State
}

func familyOf(t clause.Term) (string, bool) {
	if _, ok := t.(clause.Index); !ok {
		return "", false
	}
	root, ok := clause.Root(t).(clause.Free)
	return root.Name, ok
}

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
		return where(fn.Args, logic.Pred(fn.Name, true, ts...))
	case clause.Builtin:
		g, err := builtin(f)
		if err != nil {
			return nil, err
		}
		return where(f.Args, g)
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

// where is f read only where every entry of a map its terms read is there: a
// clause that reads an entry a map does not have is false
// (docs/CLAUSE_DESIGN.md). An entry is required before a term that reads
// under its key, and each once.
func where(ts []clause.Term, f logic.Formula) (logic.Formula, error) {
	var seen []clause.Index
	var present []logic.Formula
	var visit func(t clause.Term) error
	visit = func(t clause.Term) error {
		switch t := t.(type) {
		case clause.Index:
			if err := visit(t.Map); err != nil {
				return err
			}
			if err := visit(t.Key); err != nil {
				return err
			}
			if slices.ContainsFunc(seen, func(s clause.Index) bool { return reflect.DeepEqual(s, t) }) {
				return nil
			}
			seen = append(seen, t)
			m, err := term(t.Map)
			if err != nil {
				return err
			}
			k, err := term(t.Key)
			if err != nil {
				return err
			}
			present = append(present, logic.Pred(PredIn, false, k, m))
		case clause.Field:
			return visit(t.Of)
		case clause.App:
			for _, a := range t.Args {
				if err := visit(a); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, t := range ts {
		if err := visit(t); err != nil {
			return nil, err
		}
	}
	return logic.And(append(present, f)...), nil
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
		return logic.Pred(PredAt, false, args[0], logic.Apply(StateConstant(b))), nil
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
