package parallel

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
)

type Options struct {
	SyncEvents []core.EventID
	Files      []string
	Help       bool
}

func ParseOptions(args []string, inout *cli.ProcInout) (*Options, error) {
	flags := flag.NewFlagSet("puml-parallel", flag.ContinueOnError)
	flags.SetOutput(inout.Stderr)

	syncFlag := flags.String("sync", "", "Semicolon-separated list of synchronization events")

	flags.Usage = func() {
		fmt.Fprintf(inout.Stderr, "Usage: puml-parallel [--sync event1;event2;...] <file1.puml> [file2.puml] ...\n\n")
		fmt.Fprintf(inout.Stderr, "A tool for composing multiple PlantUML state diagrams in parallel with synchronization events.\n\n")
		fmt.Fprintf(inout.Stderr, "OPTIONS:\n")
		flags.PrintDefaults()
		fmt.Fprintf(inout.Stderr, "\nEXAMPLES:\n")
		fmt.Fprintf(inout.Stderr, "  puml-parallel file.puml\n")
		fmt.Fprintf(inout.Stderr, "  puml-parallel -sync 'insert;choose' user.puml machine.puml\n")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return &Options{Help: true}, nil
		}
		return nil, err
	}

	options := &Options{
		Files: flags.Args(),
	}

	if len(options.Files) == 0 {
		fmt.Fprintf(inout.Stderr, "error: no input files provided\n")
		flags.Usage()
		return nil, fmt.Errorf("no input files provided")
	}

	// Parse sync events
	if *syncFlag != "" {
		eventList := strings.Split(*syncFlag, ";")
		for _, event := range eventList {
			trimmed := strings.TrimSpace(event)
			if trimmed != "" {
				options.SyncEvents = append(options.SyncEvents, core.EventID(trimmed))
			}
		}
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
	var diagrams []core.Diagram

	for _, filename := range opts.Files {
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", filename, err)
		}

		parser := core.NewParser(string(content))
		diagram, err := parser.Parse()
		if err != nil {
			return fmt.Errorf("parsing file %s: %w", filename, err)
		}

		diagrams = append(diagrams, *diagram)
	}

	if len(diagrams) == 1 {
		fmt.Fprint(inout.Stdout, diagrams[0].String())
	} else {
		composite, err := core.ComposeParallel(diagrams, opts.SyncEvents)
		if err != nil {
			return fmt.Errorf("composing diagrams: %w", err)
		}
		fmt.Fprint(inout.Stdout, composite.String())
	}

	return nil
}
