package target

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

func mustBuildRefinementIR(t *testing.T) obligationir.IRRefinement {
	t.Helper()
	d := csdf.MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : a
@enduml
`)
	return obligationir.BuildRefinement(obligationir.IRRefinementModeTrace, d, d)
}

func TestValidateRefinementAcceptsEveryTarget(t *testing.T) {
	// Every backend can state the refinement obligation now that the Lean
	// translation of CSP-Prover exists.
	for _, name := range []Name{NameIRJSON, NameIsabelle, NameLean} {
		if err := ValidateRefinement(name); err != nil {
			t.Errorf("ValidateRefinement(%q) = %v, want nil", name, err)
		}
	}

	if err := ValidateRefinement("bogus"); err == nil {
		t.Error("ValidateRefinement(\"bogus\") = nil, want error")
	}
}

func TestCompileRefinementRoutesToBackend(t *testing.T) {
	ir := mustBuildRefinementIR(t)

	testCases := map[Name]string{
		NameIsabelle: "SpecProc <=T ImplProc",
		NameLean:     "theorem refines_t : refTfix SpecProc ImplProc",
	}
	for name, marker := range testCases {
		var buf bytes.Buffer
		if err := CompileRefinement(&buf, ir, name); err != nil {
			t.Fatalf("CompileRefinement(%q) error = %v", name, err)
		}
		if !strings.Contains(buf.String(), marker) {
			t.Errorf("CompileRefinement(%q) output missing %q\n%s", name, marker, buf.String())
		}
	}

	// ir-json emits decodable obligation IR JSON.
	var buf bytes.Buffer
	if err := CompileRefinement(&buf, ir, NameIRJSON); err != nil {
		t.Fatalf("CompileRefinement(%q) error = %v", NameIRJSON, err)
	}
	var got obligationir.IRRefinement
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Errorf("CompileRefinement(%q) output is not obligation IR JSON: %v", NameIRJSON, err)
	}

	// An unknown target is an error rather than a silent fallback to ir-json:
	// a caller that misspells a target must not get output it did not ask for.
	if err := CompileRefinement(io.Discard, ir, "anything-else"); err == nil {
		t.Error(`CompileRefinement("anything-else") error = nil, want an error`)
	}
}
