package csdf

import (
	"path/filepath"
	"testing"
)

func TestLoadDiagramReadsPlantUMLPNG(t *testing.T) {
	diagram, err := LoadDiagram(filepath.Join("..", "examples", "valid", "client.png"))
	if err != nil {
		t.Fatalf("LoadDiagram() error = %v", err)
	}
	if len(diagram.States) == 0 {
		t.Fatal("LoadDiagram() returned a diagram without states")
	}
}
