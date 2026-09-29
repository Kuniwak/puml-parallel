package transtable

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Conditions says how much of what holds along a way the condition of an
// outcome keeps. The zero Conditions names nothing, so a caller always says
// which it means.
type Conditions int

const (
	// ConditionsFull keeps every guard and postcondition along the way.
	ConditionsFull Conditions = iota + 1
	// ConditionsEnabling keeps what the outcome is enabled by: it leaves out
	// every postcondition whose values nothing kept after it reads. Every
	// postcondition admits some values after its step, so one of a tau step,
	// whose values are bound, is true, and leaving it out changes nothing.
	// The postcondition of the accepted event reads x', which the full
	// condition leaves free; the enabling one holds of c and x exactly where
	// the full one holds for some x', and no longer says what x' is.
	ConditionsEnabling
)

// The names of the Conditions.
const (
	ConditionsNameFull     = "full"
	ConditionsNameEnabling = "enabling"
)

var conditionsNamed = map[string]Conditions{
	ConditionsNameFull:     ConditionsFull,
	ConditionsNameEnabling: ConditionsEnabling,
}

// ParseConditions returns the Conditions named name. The names are the core's,
// so that a front end other than the command line names them the same way.
func ParseConditions(name string) (Conditions, error) {
	conds, ok := conditionsNamed[name]
	if !ok {
		return 0, fmt.Errorf("unknown conditions %q (want %s or %s)", name, ConditionsNameFull, ConditionsNameEnabling)
	}
	return conds, nil
}

// known reports whether conds is one of the Conditions.
func (conds Conditions) known() bool {
	return conds == ConditionsFull || conds == ConditionsEnabling
}

// discharge leaves out of cs, when conds asks for it, every postcondition whose
// values no conjunct kept after it reads. Only a later conjunct can read the
// values of a step, so one pass from the last conjunct leaves out, too, a
// postcondition whose values only one left out read.
func (conds Conditions) discharge(cs []conjunct) []conjunct {
	switch conds {
	case ConditionsFull:
		return cs
	case ConditionsEnabling:
	default:
		panic(fmt.Sprintf("transtable.Conditions.discharge: unknown conditions %d; Build refuses them", conds))
	}
	read := make(map[logic.Var]bool)
	kept := make([]conjunct, 0, len(cs))
	for _, c := range slices.Backward(cs) {
		if c.isPost() && !read[c.binds] {
			continue
		}
		for _, v := range logic.FreeVars(c.formula) {
			read[v] = true
		}
		kept = append(kept, c)
	}
	slices.Reverse(kept)
	return kept
}
