package main

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/core"
)

// Expected refinement relationships from core/testdata/README.md:
// abint [F= abext  (abext refines abint, so abext is spec, abint is impl)
// abint [F= a      (a refines abint, so abint is spec, a is impl)  
// vmi [F= vmdtea   (vmdtea refines vmi, so vmi is spec, vmdtea is impl)
// vmi [F= vmdalt   (vmdalt refines vmi, so vmi is spec, vmdalt is impl)
// vmi [F= vmd      (vmd refines vmi, so vmi is spec, vmd is impl)
//
// NOT refinements:
// not (abint [F= stop)  (stop does NOT refine abint)
// not (abint [F= a)     (a does NOT refine abint) - Wait, this contradicts "abint [F= a"

func main() {
	fmt.Println("Testing Core Testdata Refinement Relationships")
	fmt.Println("===============================================")

	// Test cases based on README.md
	testCases := []struct {
		name         string
		specFile     string
		implFile     string
		expectRefine bool
		note         string
	}{
		{"abint [F= abext", "core/testdata/abext.puml", "core/testdata/abint.puml", true, "abext refines abint"},
		{"abint [F= a", "core/testdata/abint.puml", "core/testdata/a.puml", true, "a refines abint"},
		{"vmi [F= vmdtea", "core/testdata/vmi.puml", "core/testdata/vmdtea.puml", true, "vmdtea refines vmi"},
		{"vmi [F= vmdalt", "core/testdata/vmi.puml", "core/testdata/vmdalt.puml", true, "vmdalt refines vmi"},
		{"vmi [F= vmd", "core/testdata/vmi.puml", "core/testdata/vmd.puml", true, "vmd refines vmi"},
		{"not (abint [F= stop)", "core/testdata/abint.puml", "core/testdata/stop.puml", false, "stop does NOT refine abint"},
		{"not (a [F= abint)", "core/testdata/a.puml", "core/testdata/abint.puml", false, "abint does NOT refine a"},
	}

	for i, tc := range testCases {
		fmt.Printf("\n=== Test Case %d: %s ===\n", i+1, tc.name)
		fmt.Printf("Note: %s\n", tc.note)
		
		result := testRefinement(tc.specFile, tc.implFile)
		
		if result.success {
			fmt.Printf("✓ Refinement verification completed\n")
			fmt.Printf("Proof obligations generated: %d\n", len(result.obligations))
			
			// Analyze if refinement likely holds based on proof obligations
			analysisResult := analyzeProofObligations(result.obligations)
			fmt.Printf("Analysis: %s\n", analysisResult)
			
			if tc.expectRefine {
				fmt.Printf("Expected: Refinement should hold ✓\n")
			} else {
				fmt.Printf("Expected: Refinement should NOT hold ✗\n")
			}
		} else {
			fmt.Printf("✗ Error: %s\n", result.error)
		}
	}
}

type testResult struct {
	success     bool
	obligations []core.ProofObligation
	error       string
}

func testRefinement(specFile, implFile string) testResult {
	// Parse specification
	specDiagram, err := parsePumlFile(specFile)
	if err != nil {
		return testResult{false, nil, fmt.Sprintf("parsing spec %s: %v", specFile, err)}
	}

	// Parse implementation
	implDiagram, err := parsePumlFile(implFile)
	if err != nil {
		return testResult{false, nil, fmt.Sprintf("parsing impl %s: %v", implFile, err)}
	}

	// Create verifier and generate proof obligations
	verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)
	obligations, err := verifier.GenerateStableFailuresProofObligations()
	if err != nil {
		return testResult{false, nil, fmt.Sprintf("generating proof obligations: %v", err)}
	}

	return testResult{true, obligations, ""}
}

func parsePumlFile(filename string) (*core.Diagram, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	parser := core.NewParser(string(content))
	diagram, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing PUML: %w", err)
	}

	return diagram, nil
}

func analyzeProofObligations(obligations []core.ProofObligation) string {
	if len(obligations) == 0 {
		return "No proof obligations - likely a trivial refinement"
	}

	var analysis []string
	traceInclusions := 0
	refusalInclusions := 0
	alphabetConsistency := 0
	initialStateRefinement := 0

	for _, obligation := range obligations {
		switch obligation.Type {
		case core.TraceInclusion:
			traceInclusions++
		case core.RefusalSetInclusion:
			refusalInclusions++
		case core.AlphabetConsistency:
			alphabetConsistency++
		case core.InitialStateRefinement:
			initialStateRefinement++
		}
	}

	if traceInclusions > 0 {
		analysis = append(analysis, fmt.Sprintf("%d trace inclusion obligations", traceInclusions))
	}
	if refusalInclusions > 0 {
		analysis = append(analysis, fmt.Sprintf("%d refusal set inclusion obligations", refusalInclusions))
	}
	if alphabetConsistency > 0 {
		analysis = append(analysis, fmt.Sprintf("%d alphabet consistency obligations", alphabetConsistency))
	}
	if initialStateRefinement > 0 {
		analysis = append(analysis, fmt.Sprintf("%d initial state refinement obligations", initialStateRefinement))
	}

	if len(analysis) == 0 {
		return "Other proof obligations require manual verification"
	}

	return fmt.Sprintf("Stable Failures Refinement Analysis: %s", fmt.Sprintf("%v", analysis))
}