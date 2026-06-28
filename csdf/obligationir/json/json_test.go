package json

import (
	"bytes"
	stdjson "encoding/json"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func mustBuildIR(t *testing.T, input string) obligationir.ObligationIR {
	t.Helper()
	d, err := csdf.ParseDiagram([]byte(input))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	return obligationir.BuildObligationIR(d)
}

func TestCompileRoundTrips(t *testing.T) {
	// Compiling the IR to JSON and decoding it back must reproduce the IR exactly.
	ir := mustBuildIR(t, `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	var buf bytes.Buffer
	if err := Compile(&buf, ir); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	var got obligationir.ObligationIR
	if err := stdjson.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid ObligationIR JSON: %v\n%s", err, buf.String())
	}
	if diff := cmp.Diff(ir, got); diff != "" {
		t.Error(diff)
	}
}
