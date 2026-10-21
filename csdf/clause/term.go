// Package clause is the internal representation of a clause, as
// docs/CLAUSE_DESIGN.md sketches it: a skeleton the tools operate on (∧, ∃,
// builtins over terms) and atoms, text with holes that a language interprets.
//
// Binding is locally nameless: a variable bound by ∃ is a de Bruijn index,
// a free variable is a name. Substitution, closing and opening are defined
// here once, so no operator renames bound variables or avoids capture by
// hand; names for bound variables are chosen only when a clause is printed.
//
// Term, Formula and Part are interfaces with one unexported marker method.
// They are sealed sum types, a deliberate exception to preferring function
// types over one-method interfaces: no function type could tell the cases
// apart, and no package outside this one adds a case.
package clause

import "fmt"

// Term is a value a clause speaks of.
type Term interface{ isTerm() }

// Free is a free variable: a name of the diagram, before the step or, when
// Primed, after it.
type Free struct {
	Name   string
	Primed bool
}

// Bound is a variable bound by an enclosing ∃: Index 0 is the nearest.
type Bound struct{ Index int }

func (Free) isTerm()  {}
func (Bound) isTerm() {}

// Index is the entry of Map at Key, m[k].
type Index struct{ Map, Key Term }

// Field is the field Name of Of, t.f.
type Field struct {
	Of   Term
	Name string
}

// App is an environment function applied to its arguments, f(t, …).
type App struct {
	Fn   string
	Args []Term
}

func (Index) isTerm() {}
func (Field) isTerm() {}
func (App) isTerm()   {}

// Root is the term a path starts from: the map of an Index, the record of a
// Field, followed down; any other term is its own root.
func Root(t Term) Term {
	switch t := t.(type) {
	case Index:
		return Root(t.Map)
	case Field:
		return Root(t.Of)
	}
	return t
}

// PrimeRoot primes the free variable a path starts from: m[k].f becomes
// m'[k].f. A path from anything else has no value after the step.
func PrimeRoot(t Term) (Term, error) {
	switch t := t.(type) {
	case Index:
		m, err := PrimeRoot(t.Map)
		if err != nil {
			return nil, err
		}
		return Index{Map: m, Key: t.Key}, nil
	case Field:
		o, err := PrimeRoot(t.Of)
		if err != nil {
			return nil, err
		}
		return Field{Of: o, Name: t.Name}, nil
	case Free:
		return Free{Name: t.Name, Primed: true}, nil
	}
	return nil, fmt.Errorf("clause.PrimeRoot: %#v is not a free variable", t)
}
