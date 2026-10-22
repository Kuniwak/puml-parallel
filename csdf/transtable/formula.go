package transtable

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

// The variables a condition speaks of:
//
//	v      the variable v of the row's state
//	k      the field k the column's event receives, as the environment
//	       offers it
//	v'     the variable v of the state an accepted event leads to
//	v@i    the variable v after the i-th tau step, which was hidden and so
//	       is taken inside the diagram
//
// A field a tau hides is bound by the ∃ hiding wrote, inside its clauses.

// clauseFormula is the formula of logic a clause states (obligation.Formula).
func clauseFormula(f clause.And) (logic.Formula, error) {
	g, err := obligation.Formula(f)
	if err != nil {
		return nil, fmt.Errorf("transtable: %w", err)
	}
	return g, nil
}

// at names the variable v after the i-th tau step, the row's own when i is
// 0.
func at(v string, i int) logic.Var {
	if i == 0 {
		return logic.Var(v)
	}
	return logic.Var(fmt.Sprintf("%s@%d", v, i))
}

func primed(v string) logic.Var { return obligation.Var(clause.Free{Name: v, Primed: true}) }

// before maps the variables of s to their values after k tau steps.
func (ix index) before(s string, k int) map[logic.Var]logic.Var {
	m := map[logic.Var]logic.Var{}
	for _, v := range ix.d.Vars[s] {
		m[logic.Var(v)] = at(v, k)
	}
	return m
}

// after maps the values after a step into s, v', to the values after the
// k-th tau step.
func (ix index) after(s string, k int) map[logic.Var]logic.Var {
	m := map[logic.Var]logic.Var{}
	for _, v := range ix.d.Vars[s] {
		m[primed(v)] = at(v, k)
	}
	return m
}
