// Package target dispatches the livelock-freedom obligation IR to a prover backend by
// target name, so every command exposes the same set of -target values and routes them
// the same way.
package target

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
	irjson "github.com/Kuniwak/puml-parallel/csdf/obligationir/json"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/lean"
)

// Output targets selectable via -target.
const (
	IRJSON   = "ir-json"
	Isabelle = "isabelle"
	Lean     = "lean"
)

// Validate reports whether name is a known target.
func Validate(name string) error {
	switch name {
	case IRJSON, Isabelle, Lean:
		return nil
	default:
		return fmt.Errorf("unknown -target %q (want ir-json, isabelle, or lean)", name)
	}
}

// Compile writes ir to w in the format named by name.
func Compile(w io.Writer, ir obligationir.ObligationIR, name string) error {
	switch name {
	case Isabelle:
		return isabelle.Compile(w, ir)
	case Lean:
		return lean.Compile(w, ir)
	default: // IRJSON
		return irjson.Compile(w, ir)
	}
}
