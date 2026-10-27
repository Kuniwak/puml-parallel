package transtable

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// UndeclaredStateError says that edges name states the diagram never
// declares. A row is a state with its name and state variables, and an
// undeclared state has neither, so such a diagram is not tabulated.
type UndeclaredStateError struct {
	// States are the undeclared states, sorted.
	States []string
}

func (e *UndeclaredStateError) Error() string {
	return fmt.Sprintf("the states %s are named by edges but never declared, so they have no name and no state variables to tabulate", strings.Join(e.States, ", "))
}

// undeclared returns the states the edges of d name that d does not declare,
// sorted, or nil when there are none.
func undeclared(d *core.Diagram) []string {
	seen := make(map[string]bool)
	var ids []string
	name := func(s string) {
		if !slices.Contains(d.States, s) && !seen[s] {
			seen[s] = true
			ids = append(ids, s)
		}
	}
	name(d.Init)
	for _, e := range d.Edges {
		name(e.Src)
		name(e.Dst)
	}
	for _, e := range d.Ends {
		name(e.Src)
	}
	slices.Sort(ids)
	return ids
}
