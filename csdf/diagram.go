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

func LoadDiagrams(files []string) ([]*Diagram, error) {
	diagrams := make([]*Diagram, 0, len(files))
	for _, file := range files {
		bs, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("csdf.LoadDiagrams: cannot read file: %w: %q", err, file)
		}

		diagram, err := ParseDiagram(bs)
		if err != nil {
			return nil, fmt.Errorf("csdf.LoadDiagrams: cannot parse file: %w: %q", err, file)
		}
		diagrams = append(diagrams, diagram)
	}
	return diagrams, nil
}

func MustLoadDiagrams(paths ...string) []*Diagram {
	diagrams, err := LoadDiagrams(paths)
	if err != nil {
		panic(err.Error())
	}
	return diagrams
}
