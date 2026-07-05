package irjson

import (
	"bytes"
	stdjson "encoding/json"
	"reflect"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func TestCompileRoundTrips(t *testing.T) {
	d := csdf.MustParse(`@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)
	ir := obligationir.BuildLivelockFree(d)

	var buf bytes.Buffer
	WriteLivelockFree(&buf, ir)

	var got obligationir.IRLivelockFree
	if err := stdjson.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid ObligationIR JSON: %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(ir, got) {
		t.Error(cmp.Diff(ir, got))
	}
}
