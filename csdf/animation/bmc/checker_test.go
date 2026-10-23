package bmc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestCheckerValidate(t *testing.T) {
	tests := map[string]struct {
		checker bmc.Checker
		wantErr bool
	}{
		"none":                      {checker: bmc.Checker{}},
		"checks and a glue command": {checker: bmc.Checker{Checks: "c.py", Evaluator: "glue"}},
		"checks and a glue module":  {checker: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue"}},
		"checks without a glue":     {checker: bmc.Checker{Checks: "c.py"}, wantErr: true},
		"a glue without checks":     {checker: bmc.Checker{EvaluatorModule: "m"}, wantErr: true},
		"both forms of the glue":    {checker: bmc.Checker{Checks: "c.py", Evaluator: "a", EvaluatorModule: "b"}, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := tt.checker.Validate()

			// Assert
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

// The message is csdfbmc's own for a walk without a glue
// (csdfbmc/tests/test_labels.py), so the REPLs and the checker say the same.
func TestCheckerWithoutAGlueSaysWhatCsdfbmcSays(t *testing.T) {
	// Execute
	err := bmc.Checker{Checks: "c.py"}.Validate()

	// Assert
	if err == nil || !strings.Contains(err.Error(), "csdfbmc reads every clause through one; pass -evaluator or -evaluator-module") {
		t.Errorf("Validate() = %v", err)
	}
}

func TestCheckerArgsOfNext(t *testing.T) {
	// Setup
	checker := bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue defs.py"}

	// Execute
	got := checker.NextArgs("d.json")

	// Assert
	want := []string{"next", "d.json", "c.py", "--evaluator-module", "csdfbmc.python_glue defs.py"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NextArgs() mismatch (-want +got):\n%s", diff)
	}
}

func TestForSourceWritesTheDiagramAsCsdfparsePrintsIt(t *testing.T) {
	// Setup
	src := []byte("@startuml\nstate \"S\" as s0\ns0 : n\n[*] --> s0 : @eq(n', n)\n@enduml\n")

	// Execute
	s, cleanup, err := bmc.ForSource(src, bmc.Checker{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	raw, err := os.ReadFile(s.Diagram)
	if err != nil {
		t.Fatal(err)
	}
	d, err := core.UnmarshalDiagram(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Init != "s0" || len(d.Vars["s0"]) != 1 {
		t.Errorf("the diagram read back = %+v, want s0 holding n", d)
	}
}

func TestForSourceRefusesADiagramItCannotRead(t *testing.T) {
	// Execute
	_, _, err := bmc.ForSource([]byte("@startuml\n[*] --> \n@enduml\n"), bmc.Checker{})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "bmc.ForSource") {
		t.Errorf("ForSource error = %v, want one of ForSource", err)
	}
}
