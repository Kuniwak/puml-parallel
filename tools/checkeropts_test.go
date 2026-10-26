package tools_test

import (
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

func TestDeclareCheckerOptions(t *testing.T) {
	tests := map[string]struct {
		args    []string
		want    bmc.Checker
		wantErr string
	}{
		"none":                      {args: nil, want: bmc.Checker{}},
		"checks and a glue command": {args: []string{"-checks", "c.py", "-evaluator", "glue"}, want: bmc.Checker{Checks: "c.py", Evaluator: "glue"}},
		"checks and a glue module":  {args: []string{"-checks", "c.py", "-evaluator-module", "csdfbmc.python_glue"}, want: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue"}},
		"a seed of zero":            {args: []string{"-seed", "0"}, want: bmc.Checker{Seed: seed(0)}},
		"functions in the order given": {
			args: []string{"-env", "$_0 が 正=true", "-env", "f=false"},
			want: bmc.Checker{Env: []bmc.Binding{{Name: "$_0 が 正", Value: true}, {Name: "f", Value: false}}},
		},
		"a function whose name holds =": {args: []string{"-env", "a=b=true"}, want: bmc.Checker{Env: []bmc.Binding{{Name: "a=b", Value: true}}}},
		"a seed that is not a number":   {args: []string{"-seed", "x"}, wantErr: "-seed expects an integer"},
		"a function without a value":    {args: []string{"-env", "f"}, wantErr: "-env expects NAME=true or NAME=false"},
		"a function without a name":     {args: []string{"-env", "=true"}, wantErr: "-env expects NAME=true or NAME=false"},
		"a function of no truth value":  {args: []string{"-env", "f=maybe"}, wantErr: "-env expects NAME=true or NAME=false"},
		"a capitalised truth value":     {args: []string{"-env", "f=True"}, wantErr: "-env expects NAME=true or NAME=false"},
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
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse(%q) = %v, want an error saying %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, checker); diff != "" {
				t.Errorf("checker mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func seed(n int) *int { return &n }
