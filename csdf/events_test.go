package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAllChannels(t *testing.T) {
	// Arrange
	want := []string{"in", "sync"}

	// Act
	got := AllChannels(MustLoadDiagrams("../examples/valid/in.puml"))

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCommonChannels(t *testing.T) {
	// Arrange
	// The vending machine sends showAvailable, which the user receives, and the
	// two sides name the field of showPurchasable apart; they still synchronise
	// on both.
	want := []string{"choose", "drop", "insert", "showAvailable", "showPurchasable"}

	// Act
	got := CommonChannels(MustLoadDiagrams(
		"../examples/valid/user.puml",
		"../examples/valid/vending_machine.puml",
	))

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
