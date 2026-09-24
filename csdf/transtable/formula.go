package transtable

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Var names a value a condition speaks of. The names are fixed:
//
//	x    the values of the state variables of the row's state
//	c    the parameters of the column's event, as the environment offers them
//	x'   the values of the state variables of the state an accepted event
//	     leads to
//	ci   the parameters of the event of the i-th tau step, which was hidden and
//	     so is chosen inside the diagram
//	xi   the values of the state variables after the i-th tau step
//
// The parameters are not parsed out of the event: a variable stands for them
// all, whatever they are, so that no syntax of parameters has to be fixed.
type Var string

const (
	varParams Var = "c"
	varNext   Var = "x'"
)

// valuesAfter names the values after the i-th tau step; after none, those of
// the row's state.
func valuesAfter(i int) Var {
	if i == 0 {
		return "x"
	}
	return Var(fmt.Sprintf("x%d", i))
}

// paramsOf names the parameters of the i-th tau step.
func paramsOf(i int) Var { return Var(fmt.Sprintf("c%d", i)) }

// Formula is a condition of first-order logic whose atoms are the guards and
// postconditions of the diagram. They are natural language, so they are opaque
// predicates here, applied to the values they read.
type Formula interface{ isFormula() }

// Atom applies a guard or a postcondition of the diagram to its arguments: a
// guard to the parameters of its event and the values before its step, a
// postcondition to those and the values after. A guard of an end edge has no
// event, so it reads the values before only.
type Atom struct {
	Pred csdf.Predicate
	Args []Var
}

// Not is the negation of its operand.
type Not struct{ Operand Formula }

// And is the conjunction of its conjuncts; with none it is true.
type And struct{ Conjuncts []Formula }

// Exists binds Vars in Body.
type Exists struct {
	Vars []Var
	Body Formula
}

func (Atom) isFormula()   {}
func (Not) isFormula()    {}
func (And) isFormula()    {}
func (Exists) isFormula() {}

// guardAtom applies a guard to its arguments, or is nil when the guard is true.
func guardAtom(g csdf.Predicate, args ...Var) Formula {
	if csdf.IsTrue(g) {
		return nil
	}
	return Atom{Pred: g, Args: args}
}

// freeVars reports whether each variable occurs free in f.
func freeVars(f Formula, free map[Var]bool, bound map[Var]bool) {
	switch f := f.(type) {
	case Atom:
		for _, v := range f.Args {
			if !bound[v] {
				free[v] = true
			}
		}
	case Not:
		freeVars(f.Operand, free, bound)
	case And:
		for _, c := range f.Conjuncts {
			freeVars(c, free, bound)
		}
	case Exists:
		inner := make(map[Var]bool, len(bound)+len(f.Vars))
		for v := range bound {
			inner[v] = true
		}
		for _, v := range f.Vars {
			inner[v] = true
		}
		freeVars(f.Body, free, inner)
	}
}

// closeOver conjoins conjuncts collected along k tau steps and binds the
// variables of those steps that occur, in step order: the parameters of a step
// before the values after it. It is nil when there is nothing to conjoin,
// which is true. The conjuncts are copied, so no two conditions share them.
func closeOver(k int, conjuncts []Formula) Formula {
	if len(conjuncts) == 0 {
		return nil
	}
	body := And{Conjuncts: append([]Formula(nil), conjuncts...)}

	free := make(map[Var]bool)
	freeVars(body, free, nil)
	var vars []Var
	for i := 1; i <= k; i++ {
		for _, v := range []Var{paramsOf(i), valuesAfter(i)} {
			if free[v] {
				vars = append(vars, v)
			}
		}
	}
	if len(vars) == 0 {
		return body
	}
	return Exists{Vars: vars, Body: body}
}
