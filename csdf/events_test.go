package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func loadAll(t *testing.T, paths ...string) []Diagram {
	t.Helper()
	diagrams := make([]Diagram, 0, len(paths))
	for _, path := range paths {
		diagram, err := LoadDiagram(path)
		if err != nil {
			t.Fatalf("LoadDiagram(%q) error = %v", path, err)
		}
		diagrams = append(diagrams, *diagram)
	}
	return diagrams
}

func TestAllEvents(t *testing.T) {
	// Arrange
	want := []string{"in", "sync"}

	// Act
	got := AllEvents(loadAll(t, "../examples/valid/in.puml"))

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCommonEvents(t *testing.T) {
	// Arrange
	want := []string{"choose(product)", "drop(product)", "insert(coin)"}

	// Act
	got := CommonEvents(loadAll(t,
		"../examples/valid/user.puml",
		"../examples/valid/vending_machine.puml",
	))

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
