package csdf

import "testing"

func TestDiagramStringOrdersStatesByID(t *testing.T) {
	// Setup: a map literal whose iteration order is not stable across runs.
	diagram := Diagram{
		States: map[StateID]State{
			"s2": {ID: "s2", Name: "Third"},
			"s0": {ID: "s0", Name: "First"},
			"s1": {ID: "s1", Name: "Second"},
		},
		StartEdge: StartEdge{Dst: "s0", Post: True},
	}
	want := `@startuml
state "First" as s0
state "Second" as s1
state "Third" as s2
[*] --> s0
@enduml
`

	// Execute
	got := diagram.String()

	// Assert
	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}
}

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

func TestDiagramStringIncludesStateVarTypes(t *testing.T) {
	// Setup
	diagram := Diagram{
		States: map[StateID]State{
			"s0": {
				ID:   "s0",
				Name: "Initial",
				Vars: []StateVar{
					{Name: "ready", Type: "bool"},
					{Name: "count"},
				},
			},
		},
		StartEdge: StartEdge{Dst: "s0", Post: True},
	}
	want := `@startuml
state "Initial" as s0
s0: ready ; bool
s0: count
[*] --> s0
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

func TestDiagramStringIncludesFreeFormEvent(t *testing.T) {
	diagram := Diagram{
		States: map[StateID]State{
			"s0": {ID: "s0", Name: "Initial"},
		},
		StartEdge: StartEdge{Dst: "s0", Post: True},
		Edges: []Edge{
			{Src: "s0", Dst: "s0", Event: "finish(result, status)", Guard: True, Post: True},
		},
	}
	want := `@startuml
state "Initial" as s0
[*] --> s0
s0 --> s0 : finish(result, status)
@enduml
`

	got := diagram.String()

	if got != want {
		t.Errorf("Diagram.String() = %q, want %q", got, want)
	}
}
