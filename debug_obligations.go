package main

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	// テスト: abint [F= abext
	fmt.Println("=== Testing abint [F= abext ===")
	
	// ファイル読み込み
	specContent, err := os.ReadFile("core/testdata/abint.puml")
	if err != nil {
		fmt.Printf("Error reading spec: %v\n", err)
		return
	}

	implContent, err := os.ReadFile("core/testdata/abext.puml")
	if err != nil {
		fmt.Printf("Error reading impl: %v\n", err)
		return
	}

	fmt.Printf("Spec content:\n%s\n", string(specContent))
	fmt.Printf("Impl content:\n%s\n", string(implContent))

	// パース
	specParser := core.NewParser(string(specContent))
	specDiagram, err := specParser.Parse()
	if err != nil {
		fmt.Printf("Error parsing spec: %v\n", err)
		return
	}

	implParser := core.NewParser(string(implContent))
	implDiagram, err := implParser.Parse()
	if err != nil {
		fmt.Printf("Error parsing impl: %v\n", err)
		return
	}

	fmt.Printf("Spec diagram - States: %d, Edges: %d\n", len(specDiagram.States), len(specDiagram.Edges))
	fmt.Printf("Impl diagram - States: %d, Edges: %d\n", len(implDiagram.States), len(implDiagram.Edges))

	// 図の詳細を表示
	fmt.Println("\nSpec States:")
	for id, state := range specDiagram.States {
		fmt.Printf("  %s: %s\n", id, state.Name)
	}
	
	fmt.Println("\nSpec Edges:")
	for _, edge := range specDiagram.Edges {
		fmt.Printf("  %s --%s--> %s\n", edge.Src, edge.Event.ID, edge.Dst)
	}
	
	fmt.Println("\nImpl States:")
	for id, state := range implDiagram.States {
		fmt.Printf("  %s: %s\n", id, state.Name)
	}
	
	fmt.Println("\nImpl Edges:")
	for _, edge := range implDiagram.Edges {
		fmt.Printf("  %s --%s--> %s\n", edge.Src, edge.Event.ID, edge.Dst)
	}

	// 証明課題生成
	verifier := core.NewStableFailuresVerifier(specDiagram, implDiagram)
	obligations, err := verifier.GenerateStableFailuresProofObligations()
	if err != nil {
		fmt.Printf("Error generating obligations: %v\n", err)
		return
	}

	fmt.Printf("\nGenerated %d obligations:\n", len(obligations))
	for i, obligation := range obligations {
		fmt.Printf("%d. %s: %s\n", i+1, obligation.Type, obligation.Description)
	}
}