package csdf

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// ParseDiagram parses a Composable State Diagram from raw .puml text or .png
// bytes (the embedded PlantUML source is extracted from PNG inputs).
func ParseDiagram(content []byte) (*Diagram, error) {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("reading PlantUML source: %w", err)
	}
	diagram, err := NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return diagram, nil
}

// LoadDiagram reads and parses a Composable State Diagram from a file.
func LoadDiagram(path string) (*Diagram, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	diagram, err := ParseDiagram(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return diagram, nil
}
