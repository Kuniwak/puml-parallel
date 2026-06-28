package target

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

func mustBuildIR(t *testing.T) obligationir.ObligationIR {
	t.Helper()
	d, err := csdf.ParseDiagram([]byte(`@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	return obligationir.BuildObligationIR(d)
}

func TestValidate(t *testing.T) {
	for _, name := range []string{IRJSON, Isabelle, Lean} {
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

	testCases := map[string]string{
		Isabelle: "theory Livelock_Obligation imports Main begin",
		Lean:     "theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by",
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

	// ir-json (and the default) emits decodable obligation IR JSON.
	for _, name := range []string{IRJSON, "anything-else"} {
		var buf bytes.Buffer
		if err := Compile(&buf, ir, name); err != nil {
			t.Fatalf("Compile(%q) error = %v", name, err)
		}
		var got obligationir.ObligationIR
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Errorf("Compile(%q) output is not ObligationIR JSON: %v", name, err)
		}
	}
}
