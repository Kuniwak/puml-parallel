package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	// 全てのリファインメント関係
	relationships := []struct {
		name     string
		specFile string
		implFile string
		shouldRefine bool
	}{
		{"abint [F= abext", "core/testdata/abint.puml", "core/testdata/abext.puml", true},
		{"abint [F= a", "core/testdata/abint.puml", "core/testdata/a.puml", true},
		{"vmi [F= vmdtea", "core/testdata/vmi.puml", "core/testdata/vmdtea.puml", true},
		{"vmi [F= vmdalt", "core/testdata/vmi.puml", "core/testdata/vmdalt.puml", true},
		{"vmi [F= vmd", "core/testdata/vmi.puml", "core/testdata/vmd.puml", true},
		{"not (abint [F= stop)", "core/testdata/abint.puml", "core/testdata/stop.puml", false},
		{"not (abext [F= a)", "core/testdata/abext.puml", "core/testdata/a.puml", false},
	}

	var output strings.Builder
	output.WriteString("# Stable Failures Refinement Proof Obligations\n\n")
	output.WriteString("Generated from core/testdata refinement relationships.\n\n")

	successCount := 0
	totalCount := len(relationships)

	for i, rel := range relationships {
		fmt.Printf("Processing %d/%d: %s\n", i+1, totalCount, rel.name)
		
		output.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, rel.name))
		output.WriteString(fmt.Sprintf("- **Specification:** `%s`\n", rel.specFile))
		output.WriteString(fmt.Sprintf("- **Implementation:** `%s`\n", rel.implFile))
		output.WriteString(fmt.Sprintf("- **Expected:** %s\n\n", expectationString(rel.shouldRefine)))

		// ファイル読み込み
		specContent, err := os.ReadFile(rel.specFile)
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error reading spec file:** %v\n\n", err))
			continue
		}

		implContent, err := os.ReadFile(rel.implFile)
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error reading impl file:** %v\n\n", err))
			continue
		}

		// パース
		specParser := core.NewParser(string(specContent))
		specDiagram, err := specParser.Parse()
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error parsing spec:** %v\n\n", err))
			continue
		}

		implParser := core.NewParser(string(implContent))
		implDiagram, err := implParser.Parse()
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error parsing impl:** %v\n\n", err))
			continue
		}

		// 証明課題生成
		verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)
		obligations, err := verifier.GenerateStableFailuresProofObligations()
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error generating obligations:** %v\n\n", err))
			continue
		}

		output.WriteString(fmt.Sprintf("### Proof Obligations (%d total)\n\n", len(obligations)))
		
		if len(obligations) == 0 {
			output.WriteString("*No proof obligations generated - this may indicate a trivial refinement or an issue with the implementation.*\n\n")
		} else {
			for j, obligation := range obligations {
				output.WriteString(fmt.Sprintf("#### %d.%d. %s\n\n", i+1, j+1, obligation.Type))
				output.WriteString(fmt.Sprintf("**Description:** %s\n\n", obligation.Description))
				
				if obligation.Premise != "" {
					output.WriteString(fmt.Sprintf("**Premise:** %s\n\n", obligation.Premise))
				}
				
				if obligation.Conclusion != "" {
					output.WriteString(fmt.Sprintf("**Conclusion:** %s\n\n", obligation.Conclusion))
				}
				
				// Context information
				output.WriteString(fmt.Sprintf("**Context:** Spec State: `%s`, Impl State: `%s`\n\n", 
					obligation.Context.SpecState, obligation.Context.ImplState))
			}
		}
		
		output.WriteString("---\n\n")
		successCount++
	}

	// サマリー追加
	output.WriteString(fmt.Sprintf("## Summary\n\n"))
	output.WriteString(fmt.Sprintf("Successfully processed %d out of %d refinement relationships.\n\n", successCount, totalCount))
	
	// 理論的期待値の説明
	output.WriteString("### Expected Results According to Stable Failures Semantics\n\n")
	output.WriteString("- **abint [F= abext**: Should hold (external choice refines internal choice)\n")
	output.WriteString("- **abint [F= a**: Should hold (a is more constrained than abint)\n")
	output.WriteString("- **vmi refinements**: Should hold (assuming vm* are more constrained versions)\n")
	output.WriteString("- **not (abint [F= stop)**: Should NOT hold (stop cannot do a,b while abint can)\n")
	output.WriteString("- **not (abext [F= a)**: Should NOT hold (abext can do b while a cannot)\n\n")

	// ファイル書き込み
	err := os.WriteFile("tmp/OBLIGATIONS.md", []byte(output.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		return
	}

	fmt.Printf("Proof obligations written to tmp/OBLIGATIONS.md (%d/%d successful)\n", successCount, totalCount)
}

func expectationString(shouldRefine bool) string {
	if shouldRefine {
		return "Refinement should hold"
	}
	return "Refinement should NOT hold"
}