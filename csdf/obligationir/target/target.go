// Package target dispatches the livelock-freedom obligation IR to a prover backend by
// target name, so every command exposes the same set of -target values and routes them
// the same way.
package target

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/core"
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
	return Validate(name)
}

// CompileRefinement writes ir to w in the format named by name.
func CompileRefinement(w io.Writer, ir obligationir.Refinement, name Name) error {
	if err := ir.Validate(); err != nil {
		return fmt.Errorf("target.CompileRefinement: %w", err)
	}
	switch name {
	case NameIsabelle:
		return isabelle.WriteFormulaRefinement(w, ir)
	case NameLean:
		return lean.WriteFormulaRefinement(w, ir)
	case NameIRJSON:
		return irjson.WriteRefinement(w, ir)
	default:
		return fmt.Errorf("target.CompileRefinement: unknown target %q", name)
	}
}

// Compile writes ir to w in the format named by name.
func Compile(w io.Writer, ir obligationir.LivelockFree, name Name) error {
	if err := ir.Validate(); err != nil {
		return fmt.Errorf("target.Compile: %w", err)
	}
	switch name {
	case NameIsabelle:
		return isabelle.WriteFormulaLivelockFree(w, ir)
	case NameLean:
		return lean.WriteFormulaLivelockFree(w, ir)
	case NameIRJSON:
		return irjson.WriteLivelockFree(w, ir)
	default:
		return fmt.Errorf("target.Compile: unknown target %q", name)
	}
}

// CompileDiagram states the livelock-freedom obligation of d and writes it
// to w in the format named by name.
func CompileDiagram(w io.Writer, d *core.Diagram, name Name) error {
	ir, err := obligationir.BuildFormulaLivelockFree(d)
	if err != nil {
		return fmt.Errorf("target.CompileDiagram: %w", err)
	}
	if err := Compile(w, ir, name); err != nil {
		return fmt.Errorf("target.CompileDiagram: %w", err)
	}
	return nil
}

// CompileDiagrams states spec ⊑ impl in mode and writes it to w in the
// format named by name.
func CompileDiagrams(w io.Writer, mode obligationir.IRRefinementMode, spec, impl *core.Diagram, name Name) error {
	ir, err := obligationir.BuildFormulaRefinement(mode, spec, impl)
	if err != nil {
		return fmt.Errorf("target.CompileDiagrams: %w", err)
	}
	if err := CompileRefinement(w, ir, name); err != nil {
		return fmt.Errorf("target.CompileDiagrams: %w", err)
	}
	return nil
}
