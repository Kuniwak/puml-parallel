package transtable

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// UndeclaredStateError says that edges name states the diagram never
// declares. A row is a state with its name and state variables, and an
// undeclared state has neither, so such a diagram is not tabulated.
type UndeclaredStateError struct {
	// States are the undeclared states, sorted.
	States []csdf.StateID
}

func (e *UndeclaredStateError) Error() string {
	ids := make([]string, len(e.States))
	for i, s := range e.States {
		ids[i] = string(s)
	}
	return fmt.Sprintf("the states %s are named by edges but never declared, so they have no name and no state variables to tabulate", strings.Join(ids, ", "))
}

// undeclared returns the states the edges of d name that d does not declare,
// sorted, or nil when there are none.
func undeclared(d *csdf.Diagram) []csdf.StateID {
	seen := make(map[csdf.StateID]bool)
	var ids []csdf.StateID
	name := func(s csdf.StateID) {
		if _, ok := d.States[s]; !ok && !seen[s] {
			seen[s] = true
			ids = append(ids, s)
		}
	}
	name(d.StartEdge.Dst)
	for _, e := range d.Edges {
		name(e.Src)
		name(e.Dst)
	}
	if d.EndEdge != nil {
		name(d.EndEdge.Src)
	}
	slices.Sort(ids)
	return ids
}
