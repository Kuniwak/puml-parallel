package irjson

import (
	"bytes"
	stdjson "encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func TestWriteRefinementRoundTrips(t *testing.T) {
	spec := csdf.MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : a
@enduml
`)
	impl := csdf.MustParse(`@startuml
state "t0" as t0
[*] --> t0
t0 --> t0 : a
t0 --> [*] : done
@enduml
`)

	for _, mode := range []obligationir.IRRefinementMode{
		obligationir.IRRefinementModeTrace,
		obligationir.IRRefinementModeStableFailure,
		obligationir.IRRefinementModeFailuresDivergence,
	} {
		t.Run(string(mode), func(t *testing.T) {
			ir := obligationir.BuildRefinement(mode, spec, impl)

			var buf bytes.Buffer
			if err := WriteRefinement(&buf, ir); err != nil {
				t.Fatalf("WriteRefinement() error = %v", err)
			}

			var got obligationir.IRRefinement
			if err := stdjson.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("output is not valid obligation IR JSON: %v\n%s", err, buf.String())
			}
			if !reflect.DeepEqual(ir, got) {
				t.Error(cmp.Diff(ir, got))
			}
		})
	}
}

// TestWriteRefinementWireForm pins the wire form itself: the mode, the shared
// alphabet, and the one predicate map both sides index into.
func TestWriteRefinementWireForm(t *testing.T) {
	spec := csdf.MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : a
@enduml
`)
	impl := csdf.MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : b
@enduml
`)

	var buf bytes.Buffer
	if err := WriteRefinement(&buf, obligationir.BuildRefinement(obligationir.IRRefinementModeTrace, spec, impl)); err != nil {
		t.Fatalf("WriteRefinement() error = %v", err)
	}

	want := `{"mode":"trace","alphabet":["a","b"],"predicates":{"4261170317":{"args":[],"text":"true"}},"constants":[],"spec":{"states":{"s0":{"fields":[],"line":2}},"edges":[{"src":"s0","dst":"s0","event":"a","event_params":[],"guard":4261170317,"guard_args":[],"post":4261170317,"post_args":[],"line":4}],"init":{"state":"s0","post":4261170317,"post_args":[],"line":3}},"impl":{"states":{"s0":{"fields":[],"line":2}},"edges":[{"src":"s0","dst":"s0","event":"b","event_params":[],"guard":4261170317,"guard_args":[],"post":4261170317,"post_args":[],"line":4}],"init":{"state":"s0","post":4261170317,"post_args":[],"line":3}}}
`
	if got := buf.String(); want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestWriteRefinementDoesNotEscapeHTML(t *testing.T) {
	// Predicate texts are natural language, where < and > are common; escaping
	// them only obscures the text for whoever has to formalise it.
	spec := csdf.MustParse(`@startuml
state "s0" as s0
s0: n
[*] --> s0
s0 --> s0 : a ; n > 0 ; true
@enduml
`)

	var buf bytes.Buffer
	if err := WriteRefinement(&buf, obligationir.BuildRefinement(obligationir.IRRefinementModeTrace, spec, spec)); err != nil {
		t.Fatalf("WriteRefinement() error = %v", err)
	}
	if !strings.Contains(buf.String(), `"n > 0"`) {
		t.Errorf("want the unescaped predicate text, got %s", buf.String())
	}
}
