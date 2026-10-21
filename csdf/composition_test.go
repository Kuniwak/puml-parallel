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
		[]string{"sync"},
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

func TestComposeParallelSynchronisesOnChannels(t *testing.T) {
	side := func(prefix string, e Edge) *Diagram {
		e.Src, e.Dst = StateID(prefix+"0"), StateID(prefix+"1")
		return &Diagram{
			States:    map[StateID]State{StateID(prefix + "0"): {Name: prefix + "0"}, StateID(prefix + "1"): {Name: prefix + "1"}},
			StartEdge: StartEdge{Dst: StateID(prefix + "0")},
			Edges:     []Edge{e},
		}
	}
	tests := []struct {
		name  string
		left  Edge
		right Edge
		sync  []string
		want  []Edge
	}{
		{
			name:  "fields named alike are one value",
			left:  Edge{Event: "send?x", Guard: "x > 0"},
			right: Edge{Event: "send?x", Post: "m' = x"},
			sync:  []string{"send"},
			want:  []Edge{{Src: "l0_r0", Dst: "l1_r1", Event: "send?x", Guard: "x > 0", Post: "m' = x"}},
		},
		{
			name:  "fields named apart are renamed on the right, without rewriting a clause",
			left:  Edge{Event: "send?x", Guard: "x > 0"},
			right: Edge{Event: "send?y", Guard: "y < 5 ∧ @opaque", Post: "m' = y"},
			sync:  []string{"send"},
			want:  []Edge{{Src: "l0_r0", Dst: "l1_r1", Event: "send?x", Guard: "x > 0 ∧ @[y := x] y < 5 ∧ @opaque", Post: "@[y := x] m' = y"}},
		},
		{
			name:  "a field chosen on either side is chosen",
			left:  Edge{Event: "send?x"},
			right: Edge{Event: "send!x"},
			sync:  []string{"send"},
			want:  []Edge{{Src: "l0_r0", Dst: "l1_r1", Event: "send!x"}},
		},
		{
			name:  "a sent field names the value, and the receiving side reads it through a renaming",
			left:  Edge{Event: "send?x", Guard: "x > 0"},
			right: Edge{Event: "send!y"},
			sync:  []string{"send"},
			want:  []Edge{{Src: "l0_r0", Dst: "l1_r1", Event: "send!y", Guard: "@[x := y] x > 0"}},
		},
		{
			name:  "a channel not synchronised interleaves",
			left:  Edge{Event: "send?x"},
			right: Edge{Event: "send?y"},
			want: []Edge{
				{Src: "l0_r0", Dst: "l1_r0", Event: "send?x"},
				{Src: "l0_r0", Dst: "l0_r1", Event: "send?y"},
				{Src: "l0_r1", Dst: "l1_r1", Event: "send?x"},
				{Src: "l1_r0", Dst: "l1_r1", Event: "send?y"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			composite, err := ComposeParallel2(side("l", tt.left), side("r", tt.right), tt.sync)
			if err != nil {
				t.Fatalf("ComposeParallel2() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, composite.Edges); diff != "" {
				t.Errorf("ComposeParallel2() edges mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestComposeParallelOrdersEdgesCanonically(t *testing.T) {
	type testCase struct {
		Left  string
		Right string
		Want  string
	}

	testCases := map[string]testCase{
		"interleaving events (both sides offer several events from their initial state, so the composite edges are discovered in map-iteration order)": {
			Left: `@startuml
state "l0" as l0
state "l1" as l1
state "l2" as l2
[*] --> l0
l0 --> l1 : x
l0 --> l2 : b
@enduml
`,
			Right: `@startuml
state "r0" as r0
state "r1" as r1
state "r2" as r2
[*] --> r0
r0 --> r1 : y
r0 --> r2 : a
@enduml
`,
			Want: `@startuml
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
`,
		},
		"edges sharing source, event and destination (only the predicates can order them)": {
			Left: `@startuml
state "l0" as l0
state "l1" as l1
[*] --> l0
l0 --> l1 : x ; g2 ; p1
l0 --> l1 : x ; g1 ; p2
l0 --> l1 : x ; g1 ; p1
@enduml
`,
			Right: `@startuml
state "r0" as r0
[*] --> r0
@enduml
`,
			Want: `@startuml
state "(l0, r0)" as l0_r0
state "(l1, r0)" as l1_r0
[*] --> l0_r0
l0_r0 --> l1_r0 : x ; g1 ; p1
l0_r0 --> l1_r0 : x ; g1 ; p2
l0_r0 --> l1_r0 : x ; g2 ; p1
@enduml
`,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			diagrams := []*Diagram{MustParse(testCase.Left), MustParse(testCase.Right)}

			// Execute
			composite, err := ComposeParallel(diagrams, nil)
			if err != nil {
				t.Fatalf("ComposeParallel() error = %v", err)
			}

			// Assert
			if diff := cmp.Diff(testCase.Want, composite.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestComposeParallelRefusesASynchronisationThatCannotMeanOneValuePerField(t *testing.T) {
	tests := map[string]struct {
		left, right         Edge
		leftVars, rightVars []StateVar
	}{
		"the left field is a right state variable": {
			left: Edge{Event: "send?x"}, right: Edge{Event: "send?y", Guard: "x > y"}, rightVars: []StateVar{{Name: "x"}},
		},
		"the left field is a right state variable of the destination": {
			left: Edge{Event: "send?x"}, right: Edge{Event: "send?y", Post: "x' = y"}, rightVars: []StateVar{{Name: "x"}},
		},
		"the right sent field is a left state variable": {
			left: Edge{Event: "send?x", Guard: "x > y"}, right: Edge{Event: "send!y"}, leftVars: []StateVar{{Name: "y"}}, rightVars: []StateVar{{Name: "y"}},
		},
		"a renaming onto a field the side keeps": {
			left: Edge{Event: "c?a?b"}, right: Edge{Event: "c?b!a"},
		},
		"fields of another number": {
			left: Edge{Event: "send?x"}, right: Edge{Event: "send?x?y"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			tt.left.Src, tt.left.Dst, tt.right.Src, tt.right.Dst = "l0", "l0", "r0", "r0"
			left := &Diagram{States: map[StateID]State{"l0": {Vars: tt.leftVars}}, StartEdge: StartEdge{Dst: "l0"}, Edges: []Edge{tt.left}}
			right := &Diagram{States: map[StateID]State{"r0": {Vars: tt.rightVars}}, StartEdge: StartEdge{Dst: "r0"}, Edges: []Edge{tt.right}}

			// Execute
			_, err := ComposeParallel2(left, right, []string{tt.left.Event.Channel()})

			// Assert
			if err == nil {
				t.Error("ComposeParallel2() error = nil, want the synchronisation refused")
			}
		})
	}
}

func TestComposeParallelSwapsFieldsSimultaneously(t *testing.T) {
	// Setup
	left := &Diagram{States: map[StateID]State{"l0": {}}, StartEdge: StartEdge{Dst: "l0"}, Edges: []Edge{{Src: "l0", Dst: "l0", Event: "ch?a?b"}}}
	right := &Diagram{States: map[StateID]State{"r0": {}}, StartEdge: StartEdge{Dst: "r0"}, Edges: []Edge{{Src: "r0", Dst: "r0", Event: "ch?b?a", Guard: "b < a"}}}

	// Execute
	composite, err := ComposeParallel2(left, right, []string{"ch"})

	// Assert
	if err != nil {
		t.Fatalf("ComposeParallel2() error = %v", err)
	}
	if want := Predicate("@[a := b, b := a] b < a"); composite.Edges[0].Guard != want {
		t.Errorf("guard = %q, want %q", composite.Edges[0].Guard, want)
	}
}

func TestComposeParallelChecksTheFieldsOfAChannelEvenWhereNoStateReachesThem(t *testing.T) {
	// Setup: the mismatching right edge is never taken.
	left := &Diagram{States: map[StateID]State{"l0": {}}, StartEdge: StartEdge{Dst: "l0"}, Edges: []Edge{{Src: "l0", Dst: "l0", Event: "send?x"}}}
	right := &Diagram{States: map[StateID]State{"r0": {}, "r9": {}}, StartEdge: StartEdge{Dst: "r0"}, Edges: []Edge{{Src: "r9", Dst: "r9", Event: "send?x?y"}}}

	// Execute
	_, err := ComposeParallel2(left, right, []string{"send"})

	// Assert
	if err == nil {
		t.Error("ComposeParallel2() error = nil, want send?x and send?x?y refused")
	}
}

func TestComposeParallelComposesThreeDiagramsFromTheLeft(t *testing.T) {
	// Setup
	side := func(id StateID, event Event, guard Predicate) *Diagram {
		return &Diagram{States: map[StateID]State{id: {}}, StartEdge: StartEdge{Dst: id}, Edges: []Edge{{Src: id, Dst: id, Event: event, Guard: guard}}}
	}

	// Execute
	composite, err := ComposeParallel([]*Diagram{side("a", "send?x", "x > 0"), side("b", "send?y", "y < 5"), side("c", "send?z", "z ≠ 2")}, []string{"send"})

	// Assert
	if err != nil {
		t.Fatalf("ComposeParallel() error = %v", err)
	}
	want := []Edge{{Src: "a_b_c", Dst: "a_b_c", Event: "send?x", Guard: "x > 0 ∧ @[y := x] y < 5 ∧ @[z := x] z ≠ 2"}}
	if diff := cmp.Diff(want, composite.Edges); diff != "" {
		t.Errorf("ComposeParallel() edges mismatch (-want +got):\n%s", diff)
	}
}

func TestComposeParallelRefusesTwoSidesSendingOneField(t *testing.T) {
	// Setup: c!y ‖ c!z synchronises only where y = z, which needs an equality
	// the tools cannot write without clause text.
	left := &Diagram{States: map[StateID]State{"l0": {}}, StartEdge: StartEdge{Dst: "l0"}, Edges: []Edge{{Src: "l0", Dst: "l0", Event: "send!y"}}}
	right := &Diagram{States: map[StateID]State{"r0": {}}, StartEdge: StartEdge{Dst: "r0"}, Edges: []Edge{{Src: "r0", Dst: "r0", Event: "send!z"}}}

	// Execute
	_, err := ComposeParallel2(left, right, []string{"send"})

	// Assert
	if err == nil {
		t.Error("ComposeParallel2() error = nil, want send!y and send!z refused")
	}
}
