package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"plantuml-parallel-composition/core"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [--sync event1,event2,...] <file1.puml> [file2.puml] ...\n", os.Args[0])
		os.Exit(1)
	}

	var syncEvents []core.EventID
	args := os.Args[1:]

	if len(args) >= 2 && args[0] == "--sync" {
		eventList := strings.Split(args[1], ",")
		for _, event := range eventList {
			syncEvents = append(syncEvents, core.EventID(strings.TrimSpace(event)))
		}
		args = args[2:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No input files provided\n")
		os.Exit(1)
	}

	var diagrams []core.Diagram
	for _, filename := range args {
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
			os.Exit(1)
		}

		parser := core.NewParser(string(content))
		diagram, err := parser.Parse()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing file %s: %v\n", filename, err)
			os.Exit(1)
		}

		diagrams = append(diagrams, *diagram)
	}

	if len(diagrams) == 1 {
		fmt.Print(diagrams[0].String())
	} else {
		composite, err := core.ComposeParallel(diagrams, syncEvents)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error composing diagrams: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(composite.String())
	}
}
