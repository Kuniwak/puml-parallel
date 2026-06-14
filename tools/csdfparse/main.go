package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
	"io"
	"os"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s < <file.puml>\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "Parses a PlantUML state diagram from stdin and prints the parsed structure as JSON.\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  $ %s < path/to/file.puml\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  $ cat path/to/file.puml | %s\n", os.Args[0])
	}
	flag.Parse()

	if exitCode := Run(os.Stdin, os.Stdout, os.Stderr); exitCode != 0 {
		os.Exit(exitCode)
	}
}

func Run(stdin io.Reader, stdout, stderr io.Writer) int {
	// Read from standard input
	input, err := io.ReadAll(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error reading from stdin: %v\n", err)
		return 1
	}

	source, err := pngsrc.Extract(input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error reading PlantUML source: %v\n", err)
		return 1
	}

	// Parse with parser
	parser := core.NewParser(source)
	diagram, err := parser.Parse()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Parse error: %v\n", err)
		return 1
	}

	if err := json.NewEncoder(stdout).Encode(diagram); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error writing JSON: %v\n", err)
		return 1
	}

	return 0
}
