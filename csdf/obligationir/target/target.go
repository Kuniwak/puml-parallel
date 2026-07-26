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

// ValidateRefinement reports whether name is a target the refinement obligation
// can be emitted to.
func ValidateRefinement(name Name) error {
	switch name {
	case NameIRJSON, NameIsabelle:
		return nil
	case NameLean:
		return fmt.Errorf("-target lean is not available for refinement obligations yet: it is deferred until the in-house Lean translation of CSP-Prover is published (want ir-json or isabelle)")
	default:
		return fmt.Errorf("unknown -target %q (want ir-json or isabelle)", name)
	}
}

// CompileRefinement writes ir to w in the format named by name.
func CompileRefinement(w io.Writer, ir obligationir.IRRefinement, name Name) error {
	switch name {
	case NameIsabelle:
		return isabelle.WriteRefinement(w, ir)
	default: // IRJSON
		return irjson.WriteRefinement(w, ir)
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
