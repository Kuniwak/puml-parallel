package transtable

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// The variables a condition speaks of. The names are fixed:
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
const (
	// OfferedParams are the parameters of the column's event.
	OfferedParams logic.Var = "c"
	// NextValues are the values after an accepted event.
	NextValues logic.Var = "x'"
)

// ValuesAfterStep names the values after the i-th tau step; after none, those
// of the row's state.
func ValuesAfterStep(i int) logic.Var {
	if i == 0 {
		return "x"
	}
	return logic.Var(fmt.Sprintf("x%d", i))
}

// ParamsOfStep names the parameters of the event of the i-th tau step.
func ParamsOfStep(i int) logic.Var { return logic.Var(fmt.Sprintf("c%d", i)) }

// stepVars are the variables k tau steps introduce, in step order: the
// parameters of a step before the values after it.
func stepVars(k int) []logic.Var {
	vars := make([]logic.Var, 0, 2*k)
	for i := 1; i <= k; i++ {
		vars = append(vars, ParamsOfStep(i), ValuesAfterStep(i))
	}
	return vars
}

// pred applies a guard or a postcondition of the diagram to what it reads. A
// predicate the author left out, or wrote as true, is true.
func pred(p csdf.Predicate, args ...logic.Var) logic.Formula {
	if csdf.IsTrue(p) {
		return logic.True
	}
	return logic.Quoted(string(p), args...)
}
