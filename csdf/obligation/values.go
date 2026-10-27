package obligation

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/types"
)

// Value is a variable of the logic that stands for a value along a run of a
// diagram, with the type it is of: the type the diagram declares of the
// state variable or of the field (docs/CLAUSE_DESIGN.md, Types), written as
// types.Type.String writes it.
type Value struct {
	Var  logic.Var
	Type string
}

// After is the value of the state variable v of the type t after the
// i-th step of a run: v@i. No name holds @, so it meets no variable of a
// clause.
func After(v string, i int, t types.Type) Value {
	return Value{Var: AfterVar(v, i), Type: t.String()}
}

// AfterVar is the variable After names.
func AfterVar(v string, i int) logic.Var { return logic.Var(fmt.Sprintf("%s@%d", v, i)) }

// Received is the value the field k of the type t the i-th step of a run
// receives: k#i. No name holds #, so it meets no variable of a clause.
func Received(k string, i int, t types.Type) Value {
	return Value{Var: logic.Var(fmt.Sprintf("%s#%d", k, i)), Type: t.String()}
}
