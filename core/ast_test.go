package core

import "testing"

func TestDiagramStringIncludesEndEdge(t *testing.T) {
	// Setup
	diagram := Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "SKIP"},
		},
		StartEdge: StartEdge{Dst: "s0", Post: True},
		EndEdge:   &EndEdge{Src: "s0", Guard: True},
	}
	want := `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*] : true
@enduml
`

	// Execute
	got := diagram.String()

	// Assert
	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}

	// Teardown: no resources to release.
}
