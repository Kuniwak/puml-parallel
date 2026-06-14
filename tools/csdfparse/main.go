package main

import (
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
		_, _ = fmt.Fprintf(os.Stderr, "Parses a PlantUML state diagram from stdin and prints the parsed structure.\n")
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

	// Output parse result
	_, _ = fmt.Fprintln(stdout, "=== Parse Result ===")
	_, _ = fmt.Fprintf(stdout, "States: %d\n", len(diagram.States))
	for id, state := range diagram.States {
		_, _ = fmt.Fprintf(stdout, "  State %s: \"%s\"\n", id, state.Name)
		for _, v := range state.Vars {
			_, _ = fmt.Fprintf(stdout, "    var: %s\n", v)
		}
	}

	_, _ = fmt.Fprintln(stdout, "\nStart Edge:")
	_, _ = fmt.Fprintf(stdout, "  [*] --> %s\n", diagram.StartEdge.Dst)
	_, _ = fmt.Fprintf(stdout, "    Post: \"%s\"\n", diagram.StartEdge.Post)

	_, _ = fmt.Fprintf(stdout, "\nEdges: %d\n", len(diagram.Edges))
	for i, edge := range diagram.Edges {
		_, _ = fmt.Fprintf(stdout, "  Edge %d: %s --> %s\n", i+1, edge.Src, edge.Dst)
		_, _ = fmt.Fprintf(stdout, "    Event: %s", edge.Event.ID)
		if len(edge.Event.Params) > 0 {
			_, _ = fmt.Fprint(stdout, "(")
			for j, param := range edge.Event.Params {
				if j > 0 {
					_, _ = fmt.Fprint(stdout, ", ")
				}
				_, _ = fmt.Fprintf(stdout, "%s", param)
			}
			_, _ = fmt.Fprint(stdout, ")")
		}
		_, _ = fmt.Fprintln(stdout)
		_, _ = fmt.Fprintf(stdout, "    Guard: \"%s\"\n", edge.Guard)
		_, _ = fmt.Fprintf(stdout, "    Post: \"%s\"\n", edge.Post)
	}

	if diagram.EndEdge != nil {
		_, _ = fmt.Fprintln(stdout, "\nEnd Edge:")
		_, _ = fmt.Fprintf(stdout, "  %s --> [*]\n", diagram.EndEdge.Src)
		_, _ = fmt.Fprintf(stdout, "    Guard: \"%s\"\n", diagram.EndEdge.Guard)
	}

	return 0
}
