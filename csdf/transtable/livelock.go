package transtable

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// LivelockError says that a tau cycle is reachable, so the diagram may diverge.
// A diverging state never becomes stable and so refuses nothing in the
// stable-failures sense, although it accepts nothing either; a table of
// refusals would show it as never refusing. The guards are not read, so a cycle
// they would cut short is refused too.
type LivelockError struct {
	Livelock *csdf.Livelock
}

func (e *LivelockError) Error() string {
	var states []string
	for _, s := range e.Livelock.CycleStates() {
		states = append(states, string(s))
	}
	return fmt.Sprintf("the tau cycle %s is reachable, so the diagram may diverge, which a table of refusals cannot show", strings.Join(states, " -> "))
}
