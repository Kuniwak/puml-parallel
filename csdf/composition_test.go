package csdf

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestComposeParallelReturnsSingleDiagramUnchanged(t *testing.T) {
	// Setup
	want := `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*]
@enduml
`

	// Execute
	composite, err := ComposeParallel(MustLoadDiagrams("../examples/valid/skip.puml"), nil)
	if err != nil {
		t.Fatalf("ComposeParallel() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, composite.String()); diff != "" {
		t.Error(diff)
	}
}

func TestComposeParallelComposesDiagrams(t *testing.T) {
	// Setup
	want := `@startuml
state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
state "(s2, s1)" as s2_s1
state "(s2, s2)" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : sync
s2_s1 --> s2_s2 : out
@enduml
`

	// Execute
	composite, err := ComposeParallel(
		MustLoadDiagrams("../examples/valid/in.puml", "../examples/valid/out.puml"),
		[]Event{"sync"},
	)
	if err != nil {
		t.Fatalf("ComposeParallel() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, composite.String()); diff != "" {
		t.Error(diff)
	}
}

func TestComposeParallelRejectsEndEdges(t *testing.T) {
	// Setup
	left := &Diagram{
		States: map[StateID]State{
			"left": {Name: "Left"},
		},
		StartEdge: StartEdge{Dst: "left"},
		EndEdge:   &EndEdge{Src: "left"},
	}
	right := &Diagram{
		States: map[StateID]State{
			"right": {Name: "Right"},
		},
		StartEdge: StartEdge{Dst: "right"},
	}

	// Execute
	_, err := ComposeParallel([]*Diagram{left, right}, nil)

	// Assert
	if err == nil {
		t.Fatal("ComposeParallel() error = nil, want end-edge rejection")
	}
	if !strings.Contains(err.Error(), "end edges are not supported") {
		t.Errorf("ComposeParallel() error = %q, want end-edge rejection", err)
	}

	// Teardown: no resources to release.
}

func TestStatePairPreservesStateVarTypes(t *testing.T) {
	// Setup
	pair := StatePair{
		Left: StateWithID{
			ID: "left",
			State: State{
				Name: "Left",
				Vars: []StateVar{{Name: "ready", Type: "bool"}},
			},
		},
		Right: StateWithID{
			ID: "right",
			State: State{
				Name: "Right",
				Vars: []StateVar{{Name: "count", Type: "int"}},
			},
		},
	}

	// Execute
	state := pair.State()

	// Assert
	want := []StateVar{
		{Name: "ready", Type: "bool"},
		{Name: "count", Type: "int"},
	}
	if len(state.Vars) != len(want) {
		t.Fatalf("StatePair.State() vars = %#v, want %#v", state.Vars, want)
	}
	for i := range want {
		if state.Vars[i] != want[i] {
			t.Errorf("StatePair.State() vars[%d] = %#v, want %#v", i, state.Vars[i], want[i])
		}
	}

	// Teardown: no resources to release.
}

func TestComposeParallelMatchesWholeEvent(t *testing.T) {
	left := &Diagram{
		States: map[StateID]State{
			"l0": {Name: "Left 0"},
			"l1": {Name: "Left 1"},
		},
		StartEdge: StartEdge{Dst: "l0"},
		Edges: []Edge{
			{Src: "l0", Dst: "l1", Event: "send(x)", Guard: PredicateTrue, Post: PredicateTrue},
		},
	}
	right := &Diagram{
		States: map[StateID]State{
			"r0": {Name: "Right 0"},
			"r1": {Name: "Right 1"},
		},
		StartEdge: StartEdge{Dst: "r0"},
		Edges: []Edge{
			{Src: "r0", Dst: "r1", Event: "send(y)", Guard: PredicateTrue, Post: PredicateTrue},
		},
	}

	composite, err := ComposeParallel2(left, right, []Event{"send(x)"})
	if err != nil {
		t.Fatalf("ComposeParallel2() error = %v", err)
	}
	if len(composite.Edges) != 1 {
		t.Fatalf("ComposeParallel2() edges = %#v, want one unsynchronized edge", composite.Edges)
	}
	if composite.Edges[0].Event != "send(y)" {
		t.Errorf("ComposeParallel2() event = %q, want send(y)", composite.Edges[0].Event)
	}

	right.Edges[0].Event = "send(x)"
	composite, err = ComposeParallel2(left, right, []Event{"send(x)"})
	if err != nil {
		t.Fatalf("ComposeParallel2() matching event error = %v", err)
	}
	if len(composite.Edges) != 1 {
		t.Fatalf("ComposeParallel2() matching event edges = %#v, want one synchronized edge", composite.Edges)
	}
	if composite.Edges[0].Event != "send(x)" {
		t.Errorf("ComposeParallel2() synchronized event = %q, want send(x)", composite.Edges[0].Event)
	}
	if composite.Edges[0].Dst != "l1_r1" {
		t.Errorf("ComposeParallel2() synchronized destination = %q, want l1_r1", composite.Edges[0].Dst)
	}
}

func TestComposeParallelOrdersEdgesCanonically(t *testing.T) {
	// Setup: both sides offer several interleaving events from their initial
	// state, so the edges of the composite are discovered in map-iteration
	// order. The output must still be ordered by (src, event, dst).
	left := MustParse(`@startuml
state "l0" as l0
state "l1" as l1
state "l2" as l2
[*] --> l0
l0 --> l1 : x
l0 --> l2 : b
@enduml
`)
	right := MustParse(`@startuml
state "r0" as r0
state "r1" as r1
state "r2" as r2
[*] --> r0
r0 --> r1 : y
r0 --> r2 : a
@enduml
`)
	want := `@startuml
state "(l0, r0)" as l0_r0
state "(l0, r1)" as l0_r1
state "(l0, r2)" as l0_r2
state "(l1, r0)" as l1_r0
state "(l1, r1)" as l1_r1
state "(l1, r2)" as l1_r2
state "(l2, r0)" as l2_r0
state "(l2, r1)" as l2_r1
state "(l2, r2)" as l2_r2
[*] --> l0_r0
l0_r0 --> l0_r2 : a
l0_r0 --> l2_r0 : b
l0_r0 --> l1_r0 : x
l0_r0 --> l0_r1 : y
l0_r1 --> l2_r1 : b
l0_r1 --> l1_r1 : x
l0_r2 --> l2_r2 : b
l0_r2 --> l1_r2 : x
l1_r0 --> l1_r2 : a
l1_r0 --> l1_r1 : y
l2_r0 --> l2_r2 : a
l2_r0 --> l2_r1 : y
@enduml
`

	// Execute
	composite, err := ComposeParallel([]*Diagram{left, right}, nil)
	if err != nil {
		t.Fatalf("ComposeParallel() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, composite.String()); diff != "" {
		t.Error(diff)
	}
}
