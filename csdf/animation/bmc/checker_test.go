package bmc_test

import (
	"flag"
	"io"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/google/go-cmp/cmp"
)

func TestCheckerFlags(t *testing.T) {
	tests := map[string]struct {
		args    []string
		want    bmc.Checker
		wantErr bool
	}{
		"none":                      {args: nil, want: bmc.Checker{}},
		"checks and a glue command": {args: []string{"-checks", "c.py", "-evaluator", "glue"}, want: bmc.Checker{Checks: "c.py", Evaluator: "glue"}},
		"checks and a glue module":  {args: []string{"-checks", "c.py", "-evaluator-module", "csdfbmc.python_glue"}, want: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue"}},
		"checks without a glue":     {args: []string{"-checks", "c.py"}, wantErr: true},
		"a glue without checks":     {args: []string{"-evaluator-module", "m"}, wantErr: true},
		"both forms of the glue":    {args: []string{"-checks", "c.py", "-evaluator", "a", "-evaluator-module", "b"}, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			checker := bmc.DeclareFlags(fs)

			// Execute
			if err := fs.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			err := checker.Validate()

			// Assert
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, *checker); diff != "" {
					t.Errorf("checker mismatch (-want +got):\n%s", diff)
				}
			}
		})
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
