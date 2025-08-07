package main

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/puml-refinement/refinement"
)

func main() {
	// Test the tool directly
	inout := &cli.ProcInout{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	// Test case 1: abint [F= abext
	fmt.Println("=== Testing abint [F= abext ===")
	args1 := []string{"-spec", "core/testdata/abint.puml", "-impl", "core/testdata/abext.puml"}
	result1 := refinement.MainCommandByArgs(args1, inout)
	fmt.Printf("Exit code: %d\n\n", result1)

	// Test case 2: abint [F= a
	fmt.Println("=== Testing abint [F= a ===")
	args2 := []string{"-spec", "core/testdata/abint.puml", "-impl", "core/testdata/a.puml"}
	result2 := refinement.MainCommandByArgs(args2, inout)
	fmt.Printf("Exit code: %d\n\n", result2)

	// Test case 3: not (abint [F= stop)
	fmt.Println("=== Testing not (abint [F= stop) ===")
	args3 := []string{"-spec", "core/testdata/abint.puml", "-impl", "core/testdata/stop.puml"}
	result3 := refinement.MainCommandByArgs(args3, inout)
	fmt.Printf("Exit code: %d\n\n", result3)

	// Test case 4: not (abext [F= a)
	fmt.Println("=== Testing not (abext [F= a) ===")
	args4 := []string{"-spec", "core/testdata/abext.puml", "-impl", "core/testdata/a.puml"}
	result4 := refinement.MainCommandByArgs(args4, inout)
	fmt.Printf("Exit code: %d\n\n", result4)
}