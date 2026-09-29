package transtable

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Conditions says how much of what holds along a way the condition of an
// outcome keeps.
type Conditions int

const (
	// ConditionsFull keeps every guard and postcondition along the way.
	ConditionsFull Conditions = iota
	// ConditionsEnabling keeps what the outcome is enabled by: it leaves out
	// every postcondition whose values nothing kept after it reads. Every
	// postcondition admits some values after its step, so binding those
	// values makes it true, and the condition means what it did; it only no
	// longer says what the values after are.
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

// discharge leaves out of cs, when conds asks for it, every postcondition whose
// values no conjunct kept after it reads. Only a later conjunct can read the
// values of a step, so one pass from the last conjunct leaves out, too, a
// postcondition whose values only one left out read.
func (conds Conditions) discharge(cs []conjunct) []conjunct {
	if conds != ConditionsEnabling {
		return cs
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
