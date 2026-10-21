package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Env is what a diagram takes from outside to be expanded: the values of
// each type (a variable bound by ∃ ranges over the type it carries; a state
// variable or a field, until they carry types, over the type of the same
// name), the meaning
// of each atom, keyed by AtomKey, and the environment functions.
type Env struct {
	Types map[string][]Value
	Atoms map[string]func(args []Value) bool
	Funcs map[string]func(args []Value) Value
}

// values are the values of the type typ; a type the environment does not
// give is an error, never an empty type.
func (env Env) values(typ string) ([]Value, error) {
	vals, ok := env.Types[typ]
	if !ok {
		return nil, fmt.Errorf("core: no type %s in the environment", typ)
	}
	return vals, nil
}

// AtomKey is the wire text of an atom with its holes written #i, i the
// argument the hole reads.
func AtomKey(w clause.AtomWire) string {
	var b strings.Builder
	for i, t := range w.Text {
		if i > 0 {
			b.WriteString("#" + strconv.Itoa(w.Holes[i-1]))
		}
		b.WriteString(t)
	}
	return b.String()
}

// valuation gives the values of the free variables a clause reads: before
// the step by name, and after it by name when primed.
type valuation struct {
	before, after map[string]Value
}

// Holds says whether f holds when its free variables have the values before
// the step in before and, primed, the values after it in after.
func Holds(f clause.Formula, env Env, before, after map[string]Value) (bool, error) {
	return evaluator{env: env, val: valuation{before: before, after: after}}.holds(f, nil)
}

// errMissing is a term that reads an entry its map does not have: an
// instance that does not exist. A clause reading one is false
// (docs/CLAUSE_DESIGN.md).
var errMissing = errors.New("an entry the map does not have")

type evaluator struct {
	env Env
	val valuation
}

func (ev evaluator) holds(f clause.Formula, bound []Value) (bool, error) {
	switch f := f.(type) {
	case clause.And:
		for _, c := range f.Conjuncts {
			ok, err := ev.holds(c, bound)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	case clause.Exists:
		vals, err := ev.env.values(f.Type)
		if err != nil {
			return false, err
		}
		for _, v := range vals {
			ok, err := ev.holds(f.Body, append(append([]Value{}, bound...), v))
			if err != nil || ok {
				return ok, err
			}
		}
		return false, nil
	case clause.Atom:
		w, terms, err := clause.Wire(f)
		if err != nil {
			return false, err
		}
		fn, ok := ev.env.Atoms[AtomKey(w)]
		if !ok {
			return false, fmt.Errorf("core: no meaning for the atom %q", AtomKey(w))
		}
		args, err := ev.terms(terms, bound)
		if errors.Is(err, errMissing) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return fn(args), nil
	case clause.Builtin:
		args, err := ev.terms(f.Args, bound)
		if errors.Is(err, errMissing) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return builtin(f, args)
	}
	return false, fmt.Errorf("core: unknown formula %#v", f)
}

func builtin(b clause.Builtin, args []Value) (bool, error) {
	switch b.Op {
	case clause.OpEq:
		return Key(args[0]) == Key(args[1]), nil
	case clause.OpNe:
		return Key(args[0]) != Key(args[1]), nil
	case clause.OpIn:
		m, ok := args[1].(Map)
		if !ok {
			return false, fmt.Errorf("core: @in of %s, not a map", Key(args[1]))
		}
		_, in := m[Key(args[0])]
		return in, nil
	case clause.OpNotIn:
		m, ok := args[1].(Map)
		if !ok {
			return false, fmt.Errorf("core: @notin of %s, not a map", Key(args[1]))
		}
		_, in := m[Key(args[0])]
		return !in, nil
	case clause.OpEmpty:
		m, ok := args[0].(Map)
		if !ok {
			return false, fmt.Errorf("core: @empty of %s, not a map", Key(args[0]))
		}
		return len(m) == 0, nil
	case clause.OpUpdate:
		m1, ok1 := args[0].(Map)
		m0, ok0 := args[1].(Map)
		if !ok0 || !ok1 {
			return false, fmt.Errorf("core: @update of a value that is not a map")
		}
		want := Map{}
		for k, v := range m0 {
			want[k] = v
		}
		want[Key(args[2])] = args[3]
		return Key(m1) == Key(want), nil
	case clause.OpFrame:
		m1, ok1 := args[0].(Map)
		m0, ok0 := args[1].(Map)
		if !ok0 || !ok1 {
			return false, fmt.Errorf("core: @frame of a value that is not a map")
		}
		except := map[string]bool{}
		for _, k := range args[2:] {
			except[Key(k)] = true
		}
		for _, m := range []Map{m0, m1} {
			for k := range m {
				if !except[k] && Key(m0[k]) != Key(m1[k]) {
					return false, nil
				}
			}
		}
		return true, nil
	case clause.OpAll:
		m, ok := args[0].(Map)
		if !ok {
			return false, fmt.Errorf("core: @all of %s, not a map", Key(args[0]))
		}
		for _, v := range m {
			if r, ok := v.(Rec); !ok || r.State != b.State {
				return false, nil
			}
		}
		return true, nil
	case clause.OpAt:
		r, ok := args[0].(Rec)
		if !ok {
			return false, fmt.Errorf("core: @at of %s, not an instance", Key(args[0]))
		}
		return r.State == b.State, nil
	}
	return false, fmt.Errorf("core: unknown builtin %q", b.Op)
}

func (ev evaluator) terms(ts []clause.Term, bound []Value) ([]Value, error) {
	out := make([]Value, len(ts))
	for i, t := range ts {
		v, err := ev.term(t, bound)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (ev evaluator) term(t clause.Term, bound []Value) (Value, error) {
	switch t := t.(type) {
	case clause.Free:
		vals := ev.val.before
		if t.Primed {
			vals = ev.val.after
		}
		v, ok := vals[t.Name]
		if !ok {
			return nil, fmt.Errorf("core: %s has no value here", name(t))
		}
		return v, nil
	case clause.Bound:
		if t.Index >= len(bound) {
			return nil, fmt.Errorf("core: the index %d is unbound", t.Index)
		}
		return bound[len(bound)-1-t.Index], nil
	case clause.Index:
		m, err := ev.term(t.Map, bound)
		if err != nil {
			return nil, err
		}
		k, err := ev.term(t.Key, bound)
		if err != nil {
			return nil, err
		}
		mm, ok := m.(Map)
		if !ok {
			return nil, fmt.Errorf("core: %s is not a map", Key(m))
		}
		v, ok := mm[Key(k)]
		if !ok {
			return nil, fmt.Errorf("core: %s is not in %s: %w", Key(k), Key(m), errMissing)
		}
		return v, nil
	case clause.Field:
		r, err := ev.term(t.Of, bound)
		if err != nil {
			return nil, err
		}
		rr, ok := r.(Rec)
		if !ok {
			return nil, fmt.Errorf("core: %s is not an instance", Key(r))
		}
		v, ok := rr.Fields[t.Name]
		if !ok {
			return nil, fmt.Errorf("core: %s has no field %s", Key(r), t.Name)
		}
		return v, nil
	case clause.App:
		fn, ok := ev.env.Funcs[t.Fn]
		if !ok {
			return nil, fmt.Errorf("core: no environment function %s", t.Fn)
		}
		args, err := ev.terms(t.Args, bound)
		if err != nil {
			return nil, err
		}
		return fn(args), nil
	}
	return nil, fmt.Errorf("core: unknown term %#v", t)
}

func name(v clause.Free) string {
	if v.Primed {
		return v.Name + "'"
	}
	return v.Name
}
