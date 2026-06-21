package csdf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDiagramReadsPlantUMLPNG(t *testing.T) {
	p := filepath.Join("..", "examples", "valid", "client.png")
	bs, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("TestLoadDiagramReadsPlantUMLPNG: cannot read file: %q", p)
	}

	diagram, err := ParseDiagram(bs)
	if err != nil {
		t.Fatalf("LoadDiagram() error = %v", err)
	}
	if len(diagram.States) == 0 {
		t.Fatal("LoadDiagram() returned a diagram without states")
	}
}
