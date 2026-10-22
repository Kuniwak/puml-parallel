package transtable

import (
	"fmt"
	"strings"
)

// LivelockError says that a tau cycle is reachable, so the diagram may diverge.
// A diverging state never becomes stable and so refuses nothing in the
// stable-failures sense, although it accepts nothing either; a table of
// refusals would show it as never refusing. The guards are not read, so a cycle
// they would cut short is refused too.
type LivelockError struct {
	// States are the reachable states on a tau cycle, sorted.
	States []string
}

func (e *LivelockError) Error() string {
	return fmt.Sprintf("the states %s are on a reachable tau cycle, so the diagram may diverge, which a table of refusals cannot show", strings.Join(e.States, ", "))
}
