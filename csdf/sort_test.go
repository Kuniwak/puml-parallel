package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSortOrdersStatesByIDAndEdgesCanonically(t *testing.T) {
	// Setup: a hand-written diagram whose states and transitions are in
	// authoring order rather than canonical order.
	d := MustParse(`@startuml
state "Second" as s1
s1 : y
s1 : x ; Nat
state "First" as s0
[*] --> s1
s1 --> s0 : b
s1 --> s0 : a ; g
s0 --> s1 : a
s0 --> [*]
@enduml
`)
	// The declaration order of the state variables is authored meaning, so it
	// is preserved.
	want := `@startuml
state "First" as s0
state "Second" as s1
s1: y
s1: x ; Nat
[*] --> s1
s0 --> s1 : a
s1 --> s0 : a ; g
s1 --> s0 : b
s0 --> [*]
@enduml
`

	// Execute
	sorted := Sort(d)

	// Assert
	if diff := cmp.Diff(want, sorted.String()); diff != "" {
		t.Error(diff)
	}
}

func TestSortDoesNotModifyItsInput(t *testing.T) {
	// Setup
	d := MustParse(`@startuml
state "Second" as s1
state "First" as s0
[*] --> s1
s1 --> s0 : b
s0 --> s1 : a
@enduml
`)
	want := d.String()

	// Execute
	Sort(d)

	// Assert
	if diff := cmp.Diff(want, d.String()); diff != "" {
		t.Error(diff)
	}
}

func TestSortIsIdempotentOnTheExamples(t *testing.T) {
	// Setup: reprinting and reparsing a canonical diagram must be a no-op,
	// otherwise repeated formatting keeps producing diffs.
	paths := []string{
		"../examples/valid/client.puml",
		"../examples/valid/server.puml",
		"../examples/valid/vending_machine.puml",
		"../examples/valid/user.puml",
		"../examples/valid/skip.puml",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			// Setup
			once := Sort(MustLoadDiagrams(path)[0]).String()

			// Execute
			twice := Sort(MustParse(once)).String()

			// Assert
			if diff := cmp.Diff(once, twice); diff != "" {
				t.Error(diff)
			}
		})
	}
}
