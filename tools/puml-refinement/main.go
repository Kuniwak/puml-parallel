package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	var specFile, implFile string
	flag.StringVar(&specFile, "spec", "", "Path to specification PUML file")
	flag.StringVar(&implFile, "impl", "", "Path to implementation PUML file")
	flag.Parse()

	if specFile == "" || implFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: puml-refinement -spec <spec.puml> -impl <impl.puml>\n")
		os.Exit(1)
	}

	// Parse specification
	specDiagram, err := parsePumlFile(specFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing specification file %s: %v\n", specFile, err)
		os.Exit(1)
	}

	// Parse implementation
	implDiagram, err := parsePumlFile(implFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing implementation file %s: %v\n", implFile, err)
		os.Exit(1)
	}

	// Create stable failures verifier
	verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)

	// Generate proof obligations
	obligations, err := verifier.GenerateStableFailuresProofObligations()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating proof obligations: %v\n", err)
		os.Exit(1)
	}

	// Output proof obligations to stdout
	fmt.Print(core.FormatProofObligations(obligations))
}

func parsePumlFile(filename string) (*core.Diagram, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	parser := core.NewParser(string(content))
	diagram, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse PUML: %w", err)
	}

	return diagram, nil
}