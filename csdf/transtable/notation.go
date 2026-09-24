package transtable

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// The names of the notations a table can be spelled in.
const (
	// NotationNatural spells the connectives as words: and, or, not,
	// implies, exists, forall.
	NotationNatural = "natural"
	// NotationLogical spells them as symbols: ∧, ∨, ¬, →, ∃, ∀.
	NotationLogical = "logical"
)

var notations = map[string]logic.Connectives{
	NotationNatural: {
		And: " and ", Or: " or ", Not: "not ", Implies: " implies ",
		Exists: "exists ", Forall: "forall ", True: "true", False: "false",
	},
	NotationLogical: {
		And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ",
		Exists: "∃", Forall: "∀", True: "true", False: "false",
	},
}

// ParseNotation returns the notation named name. The names are the core's, so
// that a front end other than the command line spells a table the same way.
func ParseNotation(name string) (logic.Notation, error) {
	c, ok := notations[name]
	if !ok {
		return logic.Notation{}, fmt.Errorf("unknown notation %q (want %s or %s)", name, NotationNatural, NotationLogical)
	}
	n, err := logic.NewNotation(c)
	if err != nil {
		panic(fmt.Sprintf("transtable.ParseNotation: the notation %q is ill made: %v", name, err))
	}
	return n, nil
}
