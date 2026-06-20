package csdfeventscmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

func collectAllEvents(filenames []string) ([]string, error) {
	eventSet := make(map[core.Event]struct{})
	for _, filename := range filenames {
		diagram, err := loadDiagram(filename)
		if err != nil {
			return nil, err
		}
		for _, edge := range diagram.Edges {
			eventSet[edge.Event] = struct{}{}
		}
	}

	events := make([]string, 0, len(eventSet))
	for event := range eventSet {
		events = append(events, string(event))
	}
	sort.Strings(events)
	return events, nil
}

func collectCommonEvents(filenames []string) ([]string, error) {
	eventCount := make(map[core.Event]int)
	for _, filename := range filenames {
		diagram, err := loadDiagram(filename)
		if err != nil {
			return nil, err
		}
		fileEvents := make(map[core.Event]struct{})
		for _, edge := range diagram.Edges {
			fileEvents[edge.Event] = struct{}{}
		}
		for event := range fileEvents {
			eventCount[event]++
		}
	}

	total := len(filenames)
	common := make([]string, 0)
	for event, count := range eventCount {
		if count == total {
			common = append(common, string(event))
		}
	}
	sort.Strings(common)
	return common, nil
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
