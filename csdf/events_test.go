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
	// The two sides name the fields of showAvailable and showPurchasable apart,
	// and still synchronise on them.
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
