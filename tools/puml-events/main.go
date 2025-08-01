package main

import (
	"flag"
	"fmt"
	"github.com/Kuniwak/puml-parallel/core"
	"os"
	"sort"
)

func main() {
	commonFlag := flag.Bool("only-common", false, "Show only common events across all files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [-common] <file1.puml> [file2.puml] ...\n", os.Args[0])
		os.Exit(1)
	}

	if *commonFlag {
		findCommonEvents(args)
	} else {
		findAllEvents(args)
	}
}

func findAllEvents(filenames []string) {
	// Set to collect unique events
	eventSet := make(map[core.EventID]struct{})

	// Process each file
	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
			os.Exit(1)
		}

		parser := core.NewParser(string(content))
		diagram, err := parser.Parse()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", filename, err)
			os.Exit(1)
		}

		// Extract events from all edges
		for _, edge := range diagram.Edges {
			eventSet[edge.Event.ID] = struct{}{}
		}
	}

	// Convert to sorted slice for consistent output
	var events []string
	for eventID := range eventSet {
		events = append(events, string(eventID))
	}
	sort.Strings(events)

	// Output events to stdout
	for _, event := range events {
		fmt.Println(event)
	}
}

func findCommonEvents(filenames []string) {
	if len(filenames) < 2 {
		_, _ = fmt.Fprintf(os.Stderr, "Error: -common requires at least 2 files\n")
		os.Exit(1)
	}

	// Map to count occurrences of each event
	eventCount := make(map[core.EventID]int)

	// Process each file
	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
			os.Exit(1)
		}

		parser := core.NewParser(string(content))
		diagram, err := parser.Parse()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", filename, err)
			os.Exit(1)
		}

		// Set to collect unique events per file
		fileEvents := make(map[core.EventID]struct{})
		for _, edge := range diagram.Edges {
			fileEvents[edge.Event.ID] = struct{}{}
		}

		// Increment count for each unique event in this file
		for eventID := range fileEvents {
			eventCount[eventID]++
		}
	}

	// Find events that appear in all files
	var commonEvents []string
	totalFiles := len(filenames)
	for eventID, count := range eventCount {
		if count == totalFiles {
			commonEvents = append(commonEvents, string(eventID))
		}
	}

	sort.Strings(commonEvents)

	// Output common events to stdout
	for _, event := range commonEvents {
		fmt.Println(event)
	}
}
