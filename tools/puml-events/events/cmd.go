package events

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
)

type Options struct {
	OnlyCommon bool
	Files      []string
	Help       bool
}

func ParseOptions(args []string, inout *cli.ProcInout) (*Options, error) {
	flags := flag.NewFlagSet("puml-events", flag.ContinueOnError)
	flags.SetOutput(inout.Stderr)

	options := &Options{}
	flags.BoolVar(&options.OnlyCommon, "only-common", false, "Show only common events across all files")

	flags.Usage = func() {
		fmt.Fprintf(inout.Stderr, "Usage: puml-events [-only-common] <file1.puml> [file2.puml] ...\n\n")
		fmt.Fprintf(inout.Stderr, "Extract events from PlantUML state diagrams.\n\n")
		fmt.Fprintf(inout.Stderr, "OPTIONS:\n")
		flags.PrintDefaults()
		fmt.Fprintf(inout.Stderr, "\nEXAMPLES:\n")
		fmt.Fprintf(inout.Stderr, "  puml-events file.puml\n")
		fmt.Fprintf(inout.Stderr, "  puml-events -only-common user.puml machine.puml\n")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			options.Help = true
			return options, nil
		}
		return nil, err
	}

	options.Files = flags.Args()

	if len(options.Files) < 1 {
		fmt.Fprintf(inout.Stderr, "error: no input files provided\n")
		flags.Usage()
		return nil, fmt.Errorf("no input files provided")
	}

	if options.OnlyCommon && len(options.Files) < 2 {
		fmt.Fprintf(inout.Stderr, "error: -only-common requires at least 2 files\n")
		flags.Usage()
		return nil, fmt.Errorf("-only-common requires at least 2 files")
	}

	return options, nil
}

func MainCommandByArgs(args []string, inout *cli.ProcInout) int {
	opts, err := ParseOptions(args, inout)
	if err != nil {
		return 1
	}

	if opts.Help {
		return 0
	}

	if err := MainCommandByOptions(opts, inout); err != nil {
		fmt.Fprintf(inout.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func MainCommandByOptions(opts *Options, inout *cli.ProcInout) error {
	if opts.OnlyCommon {
		return findCommonEvents(opts.Files, inout)
	}
	return findAllEvents(opts.Files, inout)
}

func findAllEvents(filenames []string, inout *cli.ProcInout) error {
	// Set to collect unique events
	eventSet := make(map[core.EventID]struct{})

	// Process each file
	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", filename, err)
		}

		parser := core.NewParser(string(content))
		diagram, err := parser.Parse()
		if err != nil {
			return fmt.Errorf("parsing file %s: %w", filename, err)
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
		fmt.Fprintln(inout.Stdout, event)
	}

	return nil
}

func findCommonEvents(filenames []string, inout *cli.ProcInout) error {
	// Map to count occurrences of each event
	eventCount := make(map[core.EventID]int)

	// Process each file
	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", filename, err)
		}

		parser := core.NewParser(string(content))
		diagram, err := parser.Parse()
		if err != nil {
			return fmt.Errorf("parsing file %s: %w", filename, err)
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
		fmt.Fprintln(inout.Stdout, event)
	}

	return nil
}
