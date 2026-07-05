package csdf

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// ParseDiagram parses a Composable State Diagram from raw .puml text or .png
// bytes (the embedded PlantUML source is extracted from PNG inputs).
func ParseBytes(content []byte) (*Diagram, error) {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: reading PlantUML source: %w", err)
	}
	diagram, err := NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: parse: %w", err)
	}
	return diagram, nil
}

func Parse(content string) (*Diagram, error) {
	diagram, err := NewParser(content).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parse: parse: %w", err)
	}
	return diagram, nil
}

func MustParse(content string) *Diagram {
	d, err := Parse(content)
	if err != nil {
		panic(fmt.Errorf("csdf.MustParse: %w", err))
	}
	return d
}

func LoadDiagrams(files []string) ([]*Diagram, error) {
	diagrams := make([]*Diagram, 0, len(files))
	for _, file := range files {
		bs, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("csdf.LoadDiagrams: cannot read file: %w: %q", err, file)
		}

		diagram, err := ParseBytes(bs)
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
