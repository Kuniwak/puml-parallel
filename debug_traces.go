package main

import (
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/core"
)

func main() {
	fmt.Println("=== Debugging trace extraction ===")
	
	// abextファイルを読み込み
	implContent, err := os.ReadFile("core/testdata/abext.puml")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// パース
	implParser := core.NewParser(string(implContent))
	implDiagram, err := implParser.Parse()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Diagram has StartEdge: %+v\n", implDiagram.StartEdge)
	fmt.Printf("Initial state: %s\n", implDiagram.StartEdge.Dst)

	// 手動でトレース抽出をテスト
	fmt.Println("\nManual trace extraction:")
	fmt.Printf("Starting from: %s\n", implDiagram.StartEdge.Dst)
	
	for _, edge := range implDiagram.Edges {
		fmt.Printf("Edge: %s --%s--> %s (IsTau: %v)\n", 
			edge.Src, edge.Event.ID, edge.Dst, edge.Event.IsTau())
	}

	// StableFailuresVerifierを作成してトレース抽出
	verifier := core.NewStableFailuresVerifier(implDiagram, implDiagram)
	
	// トレース抽出メソッドは非公開なので、GenerateStableFailuresProofObligations経由でテスト
	obligations, err := verifier.GenerateStableFailuresProofObligations()
	if err != nil {
		fmt.Printf("Error generating obligations: %v\n", err)
		return
	}

	fmt.Printf("\nGenerated %d obligations\n", len(obligations))
	for i, obl := range obligations {
		fmt.Printf("%d. %s: %s\n", i+1, obl.Type, obl.Description)
	}
}