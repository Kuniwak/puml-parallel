package obligation

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Value is a variable of the logic that stands for a value along a run of a
// diagram, with the type it is of: the type named like the state variable or
// the field, the convention Enabled follows.
type Value struct {
	Var  logic.Var
	Type string
}

// After is the value of the state variable v after the i-th step of a run:
// v@i, of the type v. No name holds @, so it meets no variable of a clause.
func After(v string, i int) Value {
	return Value{Var: logic.Var(fmt.Sprintf("%s@%d", v, i)), Type: v}
}

// Received is the value the field k the i-th step of a run receives: k#i, of
// the type k. No name holds #, so it meets no variable of a clause.
func Received(k string, i int) Value {
	return Value{Var: logic.Var(fmt.Sprintf("%s#%d", k, i)), Type: k}
}
