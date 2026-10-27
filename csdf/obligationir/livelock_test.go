package obligationir_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func TestBuildLivelockFreeTellsAReachableTauCycle(t *testing.T) {
	type testCase struct {
		Source string
		Want   bool
	}

	testCases := map[string]testCase{
		"a tau self-loop at the start": {
			Source: "@startuml\nstate \"a\" as a\nstate \"b\" as b\nstate \"c\" as c\n[*] --> a\na --> a : tau\n@enduml\n",
			Want:   false,
		},
		"a tau cycle of two states after a visible edge": {
			Source: "@startuml\nstate \"a\" as a\nstate \"b\" as b\nstate \"c\" as c\n[*] --> a\na --> b : go\nb --> c : tau\nc --> b : tau\n@enduml\n",
			Want:   false,
		},
		"a tau cycle no edge reaches": {
			Source: "@startuml\nstate \"a\" as a\nstate \"b\" as b\nstate \"c\" as c\n[*] --> a\nb --> b : tau\n@enduml\n",
			Want:   true,
		},
		"a cycle that is not all tau": {
			Source: "@startuml\nstate \"a\" as a\nstate \"b\" as b\nstate \"c\" as c\n[*] --> a\na --> b : tau\nb --> a : go\n@enduml\n",
			Want:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d, err := coreio.Read(tc.Source)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			ir, err := obligationir.BuildLivelockFree(d)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if ir.Structurally != tc.Want {
				t.Errorf("Structurally = %v, want %v", ir.Structurally, tc.Want)
			}
		})
	}
}

func TestBuildLivelockFreeRefusesAStartPostReadingBeforeTheStart(t *testing.T) {
	// Setup
	post, err := clause.Parse("$n = 0")
	if err != nil {
		t.Fatal(err)
	}
	d := &core.Diagram{States: []string{"a"}, Vars: map[string][]string{"a": {"n"}}, Init: "a", InitPost: post}

	// Execute
	_, err = obligationir.BuildLivelockFree(d)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the start post reads n") {
		t.Errorf("err = %v, want one naming n", err)
	}
}

func TestFormulaLivelockFreeReadsBackFromJSON(t *testing.T) {
	// Setup
	d, err := coreio.Read("@startuml\nstate \"a\" as a\na : n ; n\n[*] --> a : $n' = 10\na --> a : tau ; @exists(k : K) ($k < $n) ; $n' = $n - 1\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}
	ir, err := obligationir.BuildLivelockFree(d)
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	b, err := json.Marshal(ir)
	if err != nil {
		t.Fatal(err)
	}
	var got obligationir.LivelockFree
	err = json.Unmarshal(b, &got)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(ir, got); diff != "" {
		t.Error(diff)
	}
}

func TestBuildLivelockFreeCarriesTheDeclaredTypes(t *testing.T) {
	// Setup
	d, err := coreio.Read("@startuml\nstate \"a\" as a\na : n ; Count\n[*] --> a : @eq(n', 0)\na --> a : put?k ; ; @eq(n', k)\nnote as channels\n  channel put : Count\nend note\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	ir, err := obligationir.BuildLivelockFree(d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]obligationir.State{{Name: "a", Vars: []obligationir.Var{{Name: "n", Type: "Count"}}}}, ir.States); diff != "" {
		t.Errorf("States (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]obligationir.Field{{Name: "k", Type: "Count"}}, ir.Edges[0].Fields); diff != "" {
		t.Errorf("Fields (-want +got):\n%s", diff)
	}
}

func TestValidateRefusesAnIROfNoDiagram(t *testing.T) {
	valid := func() obligationir.LivelockFree {
		return obligationir.LivelockFree{
			Version:  obligationir.IRVersion,
			States:   []obligationir.State{{Name: "a", Vars: []obligationir.Var{}}},
			Init:     "a",
			InitPost: logic.Tree{Op: logic.OpTrue},
			Edges:    []obligationir.Edge{{Src: "a", Dst: "a", Channel: "go", Fields: []obligationir.Field{}, Step: logic.Tree{Op: logic.OpTrue}}},
		}
	}
	testCases := map[string]func(ir *obligationir.LivelockFree){
		"another version":             func(ir *obligationir.LivelockFree) { ir.Version = 0 },
		"a state named twice":         func(ir *obligationir.LivelockFree) { ir.States = append(ir.States, ir.States[0]) },
		"an unknown start state":      func(ir *obligationir.LivelockFree) { ir.Init = "b" },
		"an edge to an unknown state": func(ir *obligationir.LivelockFree) { ir.Edges[0].Dst = "b" },
		"a field received twice": func(ir *obligationir.LivelockFree) {
			ir.Edges[0].Fields = []obligationir.Field{{Name: "k", Type: "K"}, {Name: "k", Type: "K"}}
		},
		"a sent field bound nowhere": func(ir *obligationir.LivelockFree) {
			ir.Edges[0].Fields = []obligationir.Field{{Send: true, Name: "k", Type: "K"}}
		},
		"a received field named like a variable of the source": func(ir *obligationir.LivelockFree) {
			ir.States[0].Vars = []obligationir.Var{{Name: "k", Type: "K"}}
			ir.Edges[0].Fields = []obligationir.Field{{Name: "k", Type: "K"}}
		},
		"a received field named like a variable of the destination": func(ir *obligationir.LivelockFree) {
			ir.States = append(ir.States, obligationir.State{Name: "b", Vars: []obligationir.Var{{Name: "k", Type: "K"}}})
			ir.Edges[0].Dst, ir.Edges[0].Fields = "b", []obligationir.Field{{Name: "k", Type: "K"}}
		},
		"a variable of no type": func(ir *obligationir.LivelockFree) {
			ir.States[0].Vars = []obligationir.Var{{Name: "n"}}
		},
		"a received field of no type": func(ir *obligationir.LivelockFree) {
			ir.Edges[0].Fields = []obligationir.Field{{Name: "k"}}
		},
		"a tau with a field": func(ir *obligationir.LivelockFree) {
			ir.Edges[0].Channel, ir.Edges[0].Fields = "tau", []obligationir.Field{{Name: "k"}}
		},
	}

	if err := valid().Validate(); err != nil {
		t.Fatalf("the valid IR is refused: %v", err)
	}
	for name, broken := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			ir := valid()
			broken(&ir)

			// Execute
			err := ir.Validate()

			// Assert
			if err == nil {
				t.Error("err = nil, want the IR refused")
			}
		})
	}
}
