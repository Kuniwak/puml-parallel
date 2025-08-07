package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	// 最初の2つの関係だけテスト
	relationships := []struct {
		name     string
		specFile string
		implFile string
		shouldRefine bool
	}{
		{"abint [F= abext", "core/testdata/abint.puml", "core/testdata/abext.puml", true},
		{"abint [F= a", "core/testdata/abint.puml", "core/testdata/a.puml", true},
	}

	var output strings.Builder
	output.WriteString("# Stable Failures Refinement Proof Obligations\n\n")

	for i, rel := range relationships {
		fmt.Printf("Processing: %s\n", rel.name)
		
		output.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, rel.name))
		
		// ファイル読み込み
		specContent, err := os.ReadFile(rel.specFile)
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error:** %v\n\n", err))
			continue
		}

		implContent, err := os.ReadFile(rel.implFile)
		if err != nil {
			output.WriteString(fmt.Sprintf("**Error:** %v\n\n", err))
			continue
		}

		// パース
		specParser := core.NewParser(string(specContent))
		specDiagram, err := specParser.Parse()
		if err != nil {
			output.WriteString(fmt.Sprintf("**Parse Error:** %v\n\n", err))
			continue
		}

		implParser := core.NewParser(string(implContent))
		implDiagram, err := implParser.Parse()
		if err != nil {
			output.WriteString(fmt.Sprintf("**Parse Error:** %v\n\n", err))
			continue
		}

		// 証明課題生成
		verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)
		obligations, err := verifier.GenerateStableFailuresProofObligations()
		if err != nil {
			output.WriteString(fmt.Sprintf("**Verification Error:** %v\n\n", err))
			continue
		}

		output.WriteString(fmt.Sprintf("### Generated %d Proof Obligations\n\n", len(obligations)))
		
		for j, obligation := range obligations {
			output.WriteString(fmt.Sprintf("#### %d.%d. %s\n\n", i+1, j+1, obligation.Type))
			output.WriteString(fmt.Sprintf("**Description:** %s\n\n", obligation.Description))
			if obligation.Premise != "" {
				output.WriteString(fmt.Sprintf("**Premise:** %s\n\n", obligation.Premise))
			}
			if obligation.Conclusion != "" {
				output.WriteString(fmt.Sprintf("**Conclusion:** %s\n\n", obligation.Conclusion))
			}
		}
		
		output.WriteString("---\n\n")
	}

	// ファイル書き込み
	err := os.WriteFile("tmp/OBLIGATIONS.md", []byte(output.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing: %v\n", err)
		return
	}

	fmt.Println("Proof obligations written to tmp/OBLIGATIONS.md")
}