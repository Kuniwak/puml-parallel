package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAllEvents(t *testing.T) {
	// Arrange
	want := []string{"in", "sync"}

	// Act
	got := AllEvents(MustLoadDiagrams("../examples/valid/in.puml"))

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCommonEvents(t *testing.T) {
	// Arrange
	want := []string{"choose(product)", "drop(product)", "insert(coin)"}

	// Act
	got := CommonEvents(MustLoadDiagrams(
		"../examples/valid/user.puml",
		"../examples/valid/vending_machine.puml",
	))

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
