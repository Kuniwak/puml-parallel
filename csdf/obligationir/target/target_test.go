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

func mustBuildIR(t *testing.T) obligationir.IRLivelockFree {
	t.Helper()
	d, err := csdf.ParseBytes([]byte(`@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`))
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}
	return obligationir.BuildLivelockFree(d)
}

func TestValidate(t *testing.T) {
	for _, name := range []Name{NameIRJSON, NameIsabelle, NameLean} {
		if err := Validate(name); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", name, err)
		}
	}
	if err := Validate("bogus"); err == nil {
		t.Error("Validate(\"bogus\") = nil, want error")
	}
}

func TestCompileRoutesToBackend(t *testing.T) {
	ir := mustBuildIR(t)

	testCases := map[Name]string{
		NameIsabelle: "theorem livelock_free: \"wf_on {s. reachable s} {(s', s). tau_step s s'}\"",
		NameLean:     "WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by",
	}
	for name, marker := range testCases {
		var buf bytes.Buffer
		if err := Compile(&buf, ir, name); err != nil {
			t.Fatalf("Compile(%q) error = %v", name, err)
		}
		if !strings.Contains(buf.String(), marker) {
			t.Errorf("Compile(%q) output missing %q\n%s", name, marker, buf.String())
		}
	}

	// ir-json emits decodable obligation IR JSON.
	var buf bytes.Buffer
	if err := Compile(&buf, ir, NameIRJSON); err != nil {
		t.Fatalf("Compile(%q) error = %v", NameIRJSON, err)
	}
	var got obligationir.IRLivelockFree
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Errorf("Compile(%q) output is not ObligationIR JSON: %v", NameIRJSON, err)
	}

	// An unknown target is an error rather than a silent fallback to ir-json:
	// a caller that misspells a target must not get output it did not ask for.
	if err := Compile(io.Discard, ir, "anything-else"); err == nil {
		t.Error("Compile(\"anything-else\") error = nil, want an error")
	}
}
