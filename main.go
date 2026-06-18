package main

import (
	"flag"
	"fmt"
	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
	"os"
	"strings"
)

func main() {
	syncFlag := flag.String("sync", "", "Semicolon-separated list of synchronization events")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [--sync event1;event2;...] <file1.puml> [file2.puml] ...\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	var syncEvents []core.Event
	if *syncFlag != "" {
		eventList := strings.Split(*syncFlag, ";")
		for _, event := range eventList {
			trimmed := strings.TrimSpace(event)
			if trimmed != "" {
				syncEvents = append(syncEvents, core.Event(trimmed))
			}
		}
	}

	var diagrams []core.Diagram
	for _, filename := range args {
		content, err := os.ReadFile(filename)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
			os.Exit(1)
		}

		source, err := pngsrc.Extract(content)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading PlantUML source from %s: %v\n", filename, err)
			os.Exit(1)
		}

		parser := core.NewParser(source)
		diagram, err := parser.Parse()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", filename, err)
			os.Exit(1)
		}

		diagrams = append(diagrams, *diagram)
	}

	if len(diagrams) == 1 {
		fmt.Print(diagrams[0].String())
	} else {
		composite, err := core.ComposeParallel(diagrams, syncEvents)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error composing diagrams: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(composite.String())
	}
}
