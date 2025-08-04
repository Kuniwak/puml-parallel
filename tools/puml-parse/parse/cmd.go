package parse

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
)

func MainCommandByArgs(args []string, inout *cli.ProcInout) int {
	if err := MainCommand(inout); err != nil {
		fmt.Fprintf(inout.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func MainCommand(inout *cli.ProcInout) error {
	// Read from standard input
	input, err := io.ReadAll(inout.Stdin)
	if err != nil {
		return fmt.Errorf("reading from stdin: %w", err)
	}

	// Parse with parser
	parser := core.NewParser(string(input))
	diagram, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Output parse result
	fmt.Fprintf(inout.Stdout, "=== Parse Result ===\n")
	fmt.Fprintf(inout.Stdout, "States: %d\n", len(diagram.States))
	for id, state := range diagram.States {
		fmt.Fprintf(inout.Stdout, "  State %s: \"%s\"\n", id, state.Name)
		for _, v := range state.Vars {
			fmt.Fprintf(inout.Stdout, "    var: %s\n", v)
		}
	}

	fmt.Fprintf(inout.Stdout, "\nStart Edge:\n")
	fmt.Fprintf(inout.Stdout, "  [*] --> %s\n", diagram.StartEdge.Dst)
	fmt.Fprintf(inout.Stdout, "    Post: \"%s\"\n", diagram.StartEdge.Post)

	fmt.Fprintf(inout.Stdout, "\nEdges: %d\n", len(diagram.Edges))
	for i, edge := range diagram.Edges {
		fmt.Fprintf(inout.Stdout, "  Edge %d: %s --> %s\n", i+1, edge.Src, edge.Dst)
		fmt.Fprintf(inout.Stdout, "    Event: %s", edge.Event.ID)
		if len(edge.Event.Params) > 0 {
			fmt.Fprintf(inout.Stdout, "(")
			for j, param := range edge.Event.Params {
				if j > 0 {
					fmt.Fprintf(inout.Stdout, ", ")
				}
				fmt.Fprintf(inout.Stdout, "%s", param)
			}
			fmt.Fprintf(inout.Stdout, ")")
		}
		fmt.Fprintf(inout.Stdout, "\n")
		fmt.Fprintf(inout.Stdout, "    Guard: \"%s\"\n", edge.Guard)
		fmt.Fprintf(inout.Stdout, "    Post: \"%s\"\n", edge.Post)
	}

	return nil
}
