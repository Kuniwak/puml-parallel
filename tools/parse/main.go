package main

import (
	"fmt"
	"github.com/Kuniwak/plantuml-parallel-composition/core"
	"io"
	"os"
)

func main() {
	// Read from standard input
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
		os.Exit(1)
	}

	// Parse with parser
	parser := core.NewParser(string(input))
	diagram, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	// Output parse result
	fmt.Println("=== Parse Result ===")
	fmt.Printf("States: %d\n", len(diagram.States))
	for id, state := range diagram.States {
		fmt.Printf("  State %s: \"%s\"\n", id, state.Name)
		for _, v := range state.Vars {
			fmt.Printf("    var: %s\n", v)
		}
	}

	fmt.Printf("\nStart Edge:\n")
	fmt.Printf("  [*] --> %s\n", diagram.StartEdge.Dst)
	fmt.Printf("    Post: \"%s\"\n", diagram.StartEdge.Post)

	fmt.Printf("\nEdges: %d\n", len(diagram.Edges))
	for i, edge := range diagram.Edges {
		fmt.Printf("  Edge %d: %s --> %s\n", i+1, edge.Src, edge.Dst)
		fmt.Printf("    Event: %s", edge.Event.ID)
		if len(edge.Event.Params) > 0 {
			fmt.Printf("(")
			for j, param := range edge.Event.Params {
				if j > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", param)
			}
			fmt.Printf(")")
		}
		fmt.Printf("\n")
		fmt.Printf("    Guard: \"%s\"\n", edge.Guard)
		fmt.Printf("    Post: \"%s\"\n", edge.Post)
	}
}
