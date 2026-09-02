package csdf

import (
	"path/filepath"
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
	// Diagram.String does not print line numbers, and the main thing Sort
	// rewrites is exactly those, so they are asserted by hand. Snapshotting the
	// diagram instead would share the map and the slice with the input and so
	// could not tell an in-place rewrite from a copy.
	wantPrinted := d.String()
	wantLines := map[StateID]int{"s1": 2, "s0": 3}

	// Execute
	Sort(d)

	// Assert
	if diff := cmp.Diff(wantPrinted, d.String()); diff != "" {
		t.Error(diff)
	}
	for id, line := range wantLines {
		if got := d.States[id].Line; got != line {
			t.Errorf("state %q: want line %d, got %d", id, line, got)
		}
	}
	if got := d.StartEdge.Line; got != 4 {
		t.Errorf("start edge: want line 4, got %d", got)
	}
	if got := []int{d.Edges[0].Line, d.Edges[1].Line}; !cmp.Equal([]int{5, 6}, got) {
		t.Errorf("edges: want lines [5 6], got %v", got)
	}
}

func TestSortIsIdempotentOnTheExamples(t *testing.T) {
	// Setup: reprinting and reparsing a canonical diagram must be a no-op,
	// otherwise repeated formatting keeps producing diffs. Globbing rather than
	// listing the examples means a new example is covered as soon as it lands.
	paths, err := filepath.Glob(filepath.Join("..", "examples", "valid", "*.puml"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("want at least one example, got none")
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
