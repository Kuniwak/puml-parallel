package csdf

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func mustParse(t *testing.T, input string) *Diagram {
	t.Helper()
	d, err := ParseDiagram([]byte(input))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	return d
}

func TestNormalizeMergesNondeterministicEdges(t *testing.T) {
	// Setup: s0 has two `a` edges to different destinations (nondeterminism).
	d := mustParse(t, `@startuml
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : a
s0 --> s2 : a
@enduml
`)
	want := `@startuml
state "{s0}" as s0
state "{s1, s2}" as s1_s2
[*] --> s0
s0 --> s1_s2 : a
@enduml
`

	// Execute
	normalized, err := Normalize(d)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, normalized.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNormalizeKeepsDeterministicDiagramStructure(t *testing.T) {
	// Setup: an already-deterministic diagram. Each state becomes a singleton
	// set {s}; the chain structure is preserved.
	d := mustParse(t, `@startuml
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : in
s1 --> s2 : sync
@enduml
`)
	want := `@startuml
state "{s0}" as s0
state "{s1}" as s1
state "{s2}" as s2
[*] --> s0
s0 --> s1 : in
s1 --> s2 : sync
@enduml
`

	// Execute
	normalized, err := Normalize(d)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, normalized.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNormalizeDisjoinsMergedGuardsAndPosts(t *testing.T) {
	// Setup: two `a` edges from s0 with distinct guards/posts get merged into a
	// single edge whose guard/post is a true-aware OR-join.
	d := mustParse(t, `@startuml
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : a ; g1 ; p1
s0 --> s2 : a ; g2 ; p2
@enduml
`)
	want := `@startuml
state "{s0}" as s0
state "{s1, s2}" as s1_s2
[*] --> s0
s0 --> s1_s2 : a ; g1 | g2 ; p1 | p2
@enduml
`

	// Execute
	normalized, err := Normalize(d)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, normalized.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNormalizeRejectsEndEdges(t *testing.T) {
	// Setup
	d := mustParse(t, `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*] : true
@enduml
`)

	// Execute
	_, err := Normalize(d)

	// Assert
	if err == nil {
		t.Fatal("Normalize() error = nil, want end-edge rejection")
	}
	if !strings.Contains(err.Error(), "end edges are not supported") {
		t.Errorf("Normalize() error = %q, want end-edge rejection", err)
	}
}

func TestNormalizeTakesTauClosure(t *testing.T) {
	// Setup: a τ-transition from the start state. The initial normal-form state
	// is the τ-closure {s0, s1}; the result is τ-free.
	d := mustParse(t, `@startuml
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : tau
s1 --> s2 : a
@enduml
`)
	want := `@startuml
state "{s0, s1}" as s0_s1
state "{s2}" as s2
[*] --> s0_s1
s0_s1 --> s2 : a
@enduml
`

	// Execute
	normalized, err := Normalize(d)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, normalized.String()); diff != "" {
		t.Error(diff)
	}
}
