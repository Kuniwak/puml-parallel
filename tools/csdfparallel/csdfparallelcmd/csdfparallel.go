package csdfparallelcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

func parseSyncEvents(s string) []core.Event {
	if s == "" {
		return nil
	}
	var events []core.Event
	for _, event := range strings.Split(s, ";") {
		trimmed := strings.TrimSpace(event)
		if trimmed != "" {
			events = append(events, core.Event(trimmed))
		}
	}
	return events
}

func process(files []string, sync []core.Event) (string, error) {
	var diagrams []core.Diagram
	for _, filename := range files {
		diagram, err := loadDiagram(filename)
		if err != nil {
			return "", err
		}
		diagrams = append(diagrams, *diagram)
	}

	if len(diagrams) == 1 {
		return diagrams[0].String(), nil
	}

	composite, err := core.ComposeParallel(diagrams, sync)
	if err != nil {
		return "", fmt.Errorf("composing diagrams: %w", err)
	}
	return composite.String(), nil
}

func loadDiagram(filename string) (*core.Diagram, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filename, err)
	}
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("reading PlantUML source from %s: %w", filename, err)
	}
	diagram, err := core.NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", filename, err)
	}
	return diagram, nil
}
