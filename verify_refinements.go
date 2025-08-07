package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	fmt.Println("Verifying Core Testdata Refinement Relationships")
	fmt.Println("==============================================")

	// Expected refinement relationships from README.md:
	// abint [F= abext  
	// abint [F= a      
	// vmi [F= vmdtea   
	// vmi [F= vmdalt   
	// vmi [F= vmd      
	// not (abint [F= stop)
	// not (abint [F= a) -- This seems to contradict "abint [F= a"

	testCases := []struct {
		name         string
		specFile     string
		implFile     string
		expectRefine bool
		note         string
	}{
		{"abint [F= abext", "core/testdata/abint.puml", "core/testdata/abext.puml", true, "abext should refine abint"},
		{"abint [F= a", "core/testdata/abint.puml", "core/testdata/a.puml", true, "a should refine abint"},
		{"not (abint [F= stop)", "core/testdata/abint.puml", "core/testdata/stop.puml", false, "stop should NOT refine abint"},
		{"not (a [F= abint)", "core/testdata/a.puml", "core/testdata/abint.puml", false, "abint should NOT refine a (reverse direction)"},
	}

	for i, tc := range testCases {
		fmt.Printf("\n=== Test Case %d: %s ===\n", i+1, tc.name)
		fmt.Printf("Note: %s\n", tc.note)
		
		// Parse specification
		specContent, err := os.ReadFile(tc.specFile)
		if err != nil {
			fmt.Printf("✗ Error reading spec file %s: %v\n", tc.specFile, err)
			continue
		}

		specParser := core.NewParser(string(specContent))
		specDiagram, err := specParser.Parse()
		if err != nil {
			fmt.Printf("✗ Error parsing spec file %s: %v\n", tc.specFile, err)
			continue
		}

		// Parse implementation
		implContent, err := os.ReadFile(tc.implFile)
		if err != nil {
			fmt.Printf("✗ Error reading impl file %s: %v\n", tc.implFile, err)
			continue
		}

		implParser := core.NewParser(string(implContent))
		implDiagram, err := implParser.Parse()
		if err != nil {
			fmt.Printf("✗ Error parsing impl file %s: %v\n", tc.implFile, err)
			continue
		}

		// Generate proof obligations
		verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)
		obligations, err := verifier.GenerateStableFailuresProofObligations()
		if err != nil {
			fmt.Printf("✗ Error generating proof obligations: %v\n", err)
			continue
		}

		fmt.Printf("✓ Generated %d proof obligations:\n", len(obligations))
		for j, obligation := range obligations {
			fmt.Printf("  %d. %s: %s\n", j+1, obligation.Type, obligation.Description)
			if obligation.Formula != "" {
				fmt.Printf("     Formula: %s\n", obligation.Formula)
			}
		}

		// Simple analysis
		refinementLikely := analyzeRefinementLikelihood(obligations, specDiagram, implDiagram)
		
		if tc.expectRefine {
			if refinementLikely {
				fmt.Printf("✓ Expected: Refinement should hold - Analysis suggests it likely does\n")
			} else {
				fmt.Printf("⚠ Expected: Refinement should hold - But analysis suggests it may not\n")
			}
		} else {
			if !refinementLikely {
				fmt.Printf("✓ Expected: Refinement should NOT hold - Analysis suggests it likely doesn't\n")
			} else {
				fmt.Printf("⚠ Expected: Refinement should NOT hold - But analysis suggests it might\n")
			}
		}
	}
}

func analyzeRefinementLikelihood(obligations []core.ProofObligation, spec, impl *core.Diagram) bool {
	if len(obligations) == 0 {
		return true // No obligations = trivial refinement
	}

	// Simple heuristic: if implementation has fewer states and transitions than spec,
	// refinement is more likely (implementation is more constrained)
	specStates := len(spec.States)
	specTransitions := len(spec.Transitions)
	implStates := len(impl.States)
	implTransitions := len(impl.Transitions)

	// Count alphabet sizes
	specAlphabet := make(map[string]bool)
	implAlphabet := make(map[string]bool)
	
	for _, trans := range spec.Transitions {
		if trans.Event != "tau" && trans.Event != "" {
			specAlphabet[trans.Event] = true
		}
	}
	
	for _, trans := range impl.Transitions {
		if trans.Event != "tau" && trans.Event != "" {
			implAlphabet[trans.Event] = true
		}
	}

	fmt.Printf("  Analysis: Spec(%d states, %d trans, %d events) vs Impl(%d states, %d trans, %d events)\n", 
		specStates, specTransitions, len(specAlphabet), 
		implStates, implTransitions, len(implAlphabet))

	// Simple heuristic: if impl has subset of spec's alphabet and fewer/equal complexity, likely refines
	if len(implAlphabet) <= len(specAlphabet) && implStates <= specStates {
		return true
	}

	// Check for obvious failures
	for _, obligation := range obligations {
		if strings.Contains(obligation.Description, "fail") || strings.Contains(obligation.Description, "violat") {
			return false
		}
	}

	return true // Default assumption for this simple analysis
}