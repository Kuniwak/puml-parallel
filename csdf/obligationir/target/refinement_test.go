package target

import (
	"bytes"
	"encoding/json"
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

func TestValidateRefinementDefersLean(t *testing.T) {
	for _, name := range []Name{NameIRJSON, NameIsabelle} {
		if err := ValidateRefinement(name); err != nil {
			t.Errorf("ValidateRefinement(%q) = %v, want nil", name, err)
		}
	}

	// lean is blocked on publishing the in-house Lean translation of CSP-Prover,
	// so it must be rejected saying that, not as an unknown target.
	err := ValidateRefinement(NameLean)
	if err == nil {
		t.Fatal("ValidateRefinement(\"lean\") = nil, want an error")
	}
	if !strings.Contains(err.Error(), "CSP-Prover") {
		t.Errorf("want the deferral reason in %q", err)
	}

	if err := ValidateRefinement("bogus"); err == nil {
		t.Error("ValidateRefinement(\"bogus\") = nil, want error")
	}
}

func TestCompileRefinementRoutesToBackend(t *testing.T) {
	ir := mustBuildRefinementIR(t)

	var buf bytes.Buffer
	if err := CompileRefinement(&buf, ir, NameIsabelle); err != nil {
		t.Fatalf("CompileRefinement(isabelle) error = %v", err)
	}
	if marker := "SpecProc <=T ImplProc"; !strings.Contains(buf.String(), marker) {
		t.Errorf("CompileRefinement(isabelle) output missing %q\n%s", marker, buf.String())
	}

	// ir-json (and the default) emits decodable obligation IR JSON.
	for _, name := range []Name{NameIRJSON, "anything-else"} {
		var buf bytes.Buffer
		if err := CompileRefinement(&buf, ir, name); err != nil {
			t.Fatalf("CompileRefinement(%q) error = %v", name, err)
		}
		var got obligationir.IRRefinement
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Errorf("CompileRefinement(%q) output is not obligation IR JSON: %v", name, err)
		}
	}
}
