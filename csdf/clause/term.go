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
