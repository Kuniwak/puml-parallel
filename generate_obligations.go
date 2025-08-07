package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	// リファインメント関係の定義（README.mdより）
	relationships := []struct {
		name     string
		specFile string
		implFile string
		shouldRefine bool
	}{
		// 成功すべきリファインメント
		{"abint [F= abext", "core/testdata/abint.puml", "core/testdata/abext.puml", true},
		{"abint [F= a", "core/testdata/abint.puml", "core/testdata/a.puml", true},
		{"vmi [F= vmdtea", "core/testdata/vmi.puml", "core/testdata/vmdtea.puml", true},
		{"vmi [F= vmdalt", "core/testdata/vmi.puml", "core/testdata/vmdalt.puml", true},
		{"vmi [F= vmd", "core/testdata/vmi.puml", "core/testdata/vmd.puml", true},
		
		// 失敗すべきリファインメント
		{"not (abint [F= stop)", "core/testdata/abint.puml", "core/testdata/stop.puml", false},
		{"not (abext [F= a)", "core/testdata/abext.puml", "core/testdata/a.puml", false},
	}

	var output strings.Builder
	output.WriteString("# Stable Failures Refinement Proof Obligations\n\n")
	output.WriteString("Generated from core/testdata refinement relationships.\n\n")

	for i, rel := range relationships {
		fmt.Printf("Processing: %s\n", rel.name)
		
		output.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, rel.name))
		output.WriteString(fmt.Sprintf("- Specification: `%s`\n", rel.specFile))
		output.WriteString(fmt.Sprintf("- Implementation: `%s`\n", rel.implFile))
		output.WriteString(fmt.Sprintf("- Expected: %s\n\n", expectationString(rel.shouldRefine)))

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
			output.WriteString("*No proof obligations generated - likely a trivial refinement.*\n\n")
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
				output.WriteString(fmt.Sprintf("**Context:** Spec State: %s, Impl State: %s\n\n", 
					obligation.Context.SpecState, obligation.Context.ImplState))
			}
		}
		
		output.WriteString("---\n\n")
	}

	// ファイル書き込み
	err := os.WriteFile("tmp/OBLIGATIONS.md", []byte(output.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		return
	}

	fmt.Println("Proof obligations written to tmp/OBLIGATIONS.md")
}

func expectationString(shouldRefine bool) string {
	if shouldRefine {
		return "Refinement should hold"
	}
	return "Refinement should NOT hold"
}