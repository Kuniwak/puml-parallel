// Package target dispatches the livelock-freedom obligation IR to a prover backend by
// target name, so every command exposes the same set of -target values and routes them
// the same way.
package target

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/irjson"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/lean"
)

type Name string

// Output targets selectable via -target.
const (
	NameIRJSON   Name = "ir-json"
	NameIsabelle Name = "isabelle"
	NameLean     Name = "lean"
)

// Validate reports whether name is a known target.
func Validate(name Name) error {
	switch name {
	case NameIRJSON, NameIsabelle, NameLean:
		return nil
	default:
		return fmt.Errorf("unknown -target %q (want ir-json, isabelle, or lean)", name)
	}
}

// Compile writes ir to w in the format named by name.
func Compile(w io.Writer, ir obligationir.IRLivelockFree, name Name) error {
	switch name {
	case NameIsabelle:
		isabelle.WriteLivelockFree(w, ir)
		return nil
	case NameLean:
		lean.WriteLivelockFree(w, ir)
		return nil
	default: // IRJSON
		return irjson.WriteLivelockFree(w, ir)
	}
}
