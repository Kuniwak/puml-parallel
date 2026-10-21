package logic

import "fmt"

// The variables an obligation or a condition names an event's parameters by,
// shared by the tools that write them so that one predicate is applied to
// the same arguments everywhere. A variable stands for all the parameters of
// one event, whatever they are, so that no syntax of parameters is needed.
const (
	// OfferedParams are the parameters of an event the environment offers.
	OfferedParams Var = "c"
	// ChosenParams are the parameters a tau out of a state chooses.
	ChosenParams Var = "c'"
)

// ParamsOfStep names the parameters of the event of the i-th step.
func ParamsOfStep(i int) Var { return Var(fmt.Sprintf("c%d", i)) }
