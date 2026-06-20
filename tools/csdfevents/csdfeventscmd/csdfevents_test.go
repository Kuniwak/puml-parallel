package csdfeventscmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCollectAllEvents(t *testing.T) {
	// Arrange
	want := []string{"in", "sync"}

	// Act
	got, err := collectAllEvents([]string{"../../../examples/valid/in.puml"})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCollectCommonEvents(t *testing.T) {
	// Arrange
	want := []string{"choose(product)", "drop(product)", "insert(coin)"}

	// Act
	got, err := collectCommonEvents([]string{
		"../../../examples/valid/user.puml",
		"../../../examples/valid/vending_machine.puml",
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
