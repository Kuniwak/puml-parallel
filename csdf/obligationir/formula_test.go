package obligationir_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func TestBuildFormulaLivelockFreeTellsAReachableTauCycle(t *testing.T) {
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
			ir, err := obligationir.BuildFormulaLivelockFree(d)

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

func TestBuildFormulaLivelockFreeRefusesAStartPostReadingBeforeTheStart(t *testing.T) {
	// Setup
	d, err := coreio.Read("@startuml\nstate \"a\" as a\na : n\n[*] --> a : $n = 0\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err = obligationir.BuildFormulaLivelockFree(d)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the start post reads n") {
		t.Errorf("err = %v, want one naming n", err)
	}
}

func TestFormulaLivelockFreeReadsBackFromJSON(t *testing.T) {
	// Setup
	d, err := coreio.Read("@startuml\nstate \"a\" as a\na : n\n[*] --> a : $n' = 10\na --> a : tau ; @exists(k : K) ($k < $n) ; $n' = $n - 1\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}
	ir, err := obligationir.BuildFormulaLivelockFree(d)
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
