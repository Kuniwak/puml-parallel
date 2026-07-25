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

// TestCompileTauSelfLoopWithVars pins the wire form itself, not just its
// round-trip: the predicate map is keyed by the same base-36-rendered ids the
// isabelle and lean skeletons name their definitions after, and the init
// predicate carries the start state's variables.
func TestCompileTauSelfLoopWithVars(t *testing.T) {
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
a: n ; nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	// Ids 3167241880 and 3618715515 are 1gdozh4 and 1nuhmrf in base 36, the
	// suffixes the isabelle skeleton names pred_1gdozh4 and pred_1nuhmrf after.
	want := `{"structurally":false,"predicates":{"1429597351":{"args":[{"name":"n","type":"nat","primed":false}],"text":"true"},"3167241880":{"args":[{"name":"n","type":"nat","primed":false}],"text":"n > 0"},"3618715515":{"args":[{"name":"n","type":"nat","primed":false},{"name":"n","type":"nat","primed":true}],"text":"n' = n - 1"}},"states":{"a":{"fields":[{"name":"n","type":"nat"}],"line":2}},"constants":[],"edges":[{"src":"a","dst":"a","event":"tau","event_params":[],"guard":3167241880,"post":3618715515,"line":5}],"init":{"state":"a","post":1429597351,"line":4}}
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}
