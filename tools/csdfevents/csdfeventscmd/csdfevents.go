package csdfeventscmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("csdfevents", flag.ContinueOnError)
	flags.SetOutput(stderr)
	onlyCommon := flags.Bool("only-common", false, "Show only common events across all files")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	files := flags.Args()
	if len(files) < 1 {
		_, _ = fmt.Fprintf(stderr, "Usage: csdfevents [-only-common] <file1.puml> [file2.puml] ...\n")
		return 1
	}

	var events []string
	var err error
	if *onlyCommon {
		if len(files) < 2 {
			_, _ = fmt.Fprintf(stderr, "Error: -only-common requires at least 2 files\n")
			return 1
		}
		events, err = collectCommonEvents(files)
	} else {
		events, err = collectAllEvents(files)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	for _, event := range events {
		_, _ = fmt.Fprintln(stdout, event)
	}
	return 0
}

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
