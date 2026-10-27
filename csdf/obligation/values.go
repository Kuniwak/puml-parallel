package obligation

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Value is a variable of the logic that stands for a value along a run of a
// diagram, with the type it is of: the type the diagram declares of the
// state variable or of the field (docs/CLAUSE_DESIGN.md, Types), written as
// types.Type.String writes it.
type Value struct {
	Var  logic.Var
	Type string
}

// After is the value of the state variable v of the type typ after the
// i-th step of a run: v@i. No name holds @, so it meets no variable of a
// clause.
func After(v string, i int, typ string) Value {
	return Value{Var: AfterVar(v, i), Type: typ}
}

// AfterVar is the variable After names.
func AfterVar(v string, i int) logic.Var { return logic.Var(fmt.Sprintf("%s@%d", v, i)) }

// Received is the value the field k of the type typ the i-th step of a run
// receives: k#i. No name holds #, so it meets no variable of a clause.
func Received(k string, i int, typ string) Value {
	return Value{Var: logic.Var(fmt.Sprintf("%s#%d", k, i)), Type: typ}
}

// VarType is the type d declares of its state variable v, "" when it
// declares none.
func VarType(d core.Diagram, v string) string {
	if t, ok := d.Types[v]; ok {
		return t.String()
	}
	return ""
}

// FieldType is the type d declares of the field k e receives, by its
// position on e's channel, "" when it declares none.
func FieldType(d core.Diagram, e core.Event, k string) string {
	i := slices.IndexFunc(e.Fields, func(f core.Field) bool { return f.Name == k })
	if decl := d.Channels[e.Channel]; i >= 0 && i < len(decl) {
		return decl[i].String()
	}
	return ""
}
