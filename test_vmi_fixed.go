package main

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	fmt.Println("=== Testing Fixed VMI Trace Extraction ===")
	
	// Test vmi.puml
	vmiContent, err := os.ReadFile("core/testdata/vmi.puml")
	if err != nil {
		fmt.Printf("Error reading vmi.puml: %v\n", err)
		return
	}

	vmiParser := core.NewParser(string(vmiContent))
	vmiDiagram, err := vmiParser.Parse()
	if err != nil {
		fmt.Printf("Error parsing vmi.puml: %v\n", err)
		return
	}

	fmt.Printf("VMI Diagram - States: %d, Edges: %d\n", len(vmiDiagram.States), len(vmiDiagram.Edges))
	
	// Test vmdtea.puml
	vmdteaContent, err := os.ReadFile("core/testdata/vmdtea.puml")
	if err != nil {
		fmt.Printf("Error reading vmdtea.puml: %v\n", err)
		return
	}

	vmdteaParser := core.NewParser(string(vmdteaContent))
	vmdteaDiagram, err := vmdteaParser.Parse()
	if err != nil {
		fmt.Printf("Error parsing vmdtea.puml: %v\n", err)
		return
	}

	fmt.Printf("VMDtea Diagram - States: %d, Edges: %d\n", len(vmdteaDiagram.States), len(vmdteaDiagram.Edges))

	// Test trace extraction with cycle detection
	fmt.Println("\n=== Testing VMI Trace Extraction ===")
	verifier := core.NewStableFailuresVerifier(vmiDiagram, vmdteaDiagram)
	
	obligations, err := verifier.GenerateStableFailuresProofObligations()
	if err != nil {
		fmt.Printf("Error generating obligations: %v\n", err)
		return
	}

	fmt.Printf("Generated %d proof obligations successfully!\n", len(obligations))
	
	for i, obligation := range obligations {
		fmt.Printf("%d. %s: %s\n", i+1, obligation.Type, obligation.Description)
	}
	
	fmt.Println("\n=== Success! No infinite loops ===")
}