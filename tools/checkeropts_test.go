package tools_test

import (
	"flag"
	"io"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

func TestDeclareCheckerOptions(t *testing.T) {
	tests := map[string]struct {
		args []string
		want bmc.Checker
	}{
		"none":                      {args: nil, want: bmc.Checker{}},
		"checks and a glue command": {args: []string{"-checks", "c.py", "-evaluator", "glue"}, want: bmc.Checker{Checks: "c.py", Evaluator: "glue"}},
		"checks and a glue module":  {args: []string{"-checks", "c.py", "-evaluator-module", "csdfbmc.python_glue"}, want: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			var checker bmc.Checker
			tools.DeclareCheckerOptions(fs, &checker)

			// Execute
			err := fs.Parse(tt.args)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, checker); diff != "" {
				t.Errorf("checker mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
