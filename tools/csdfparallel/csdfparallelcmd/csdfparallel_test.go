package csdfparallelcmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/core"
	"github.com/google/go-cmp/cmp"
)

func TestProcessSingleFile(t *testing.T) {
	// Arrange
	want := `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*] : true
@enduml
`

	// Act
	got, err := process([]string{"../../../examples/valid/skip.puml"}, nil)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestProcessCompose(t *testing.T) {
	// Arrange
	want := `@startuml
state "s0 || s0" as s0_s0
state "s1 || s0" as s1_s0
state "s2 || s1" as s2_s1
state "s2 || s2" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : sync
s2_s1 --> s2_s2 : out
@enduml
`

	// Act
	got, err := process(
		[]string{"../../../examples/valid/in.puml", "../../../examples/valid/out.puml"},
		[]core.Event{"sync"},
	)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
