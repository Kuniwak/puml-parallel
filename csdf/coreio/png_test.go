package coreio_test

import (
	"os"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
)

func TestReadBytesReadsAPlantUMLPNG(t *testing.T) {
	// Arrange
	bs, err := os.ReadFile("../../examples/valid/client.png")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Act
	d, err := coreio.ReadBytes(bs)

	// Assert
	if err != nil {
		t.Fatalf("coreio.ReadBytes() error = %v", err)
	}
	if len(d.States) == 0 {
		t.Fatal("coreio.ReadBytes() gave a diagram without states")
	}
}
