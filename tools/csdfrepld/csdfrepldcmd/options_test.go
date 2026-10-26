package csdfrepldcmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncTakesTheEnvironment(t *testing.T) {
	tests := map[string]struct {
		args    []string
		want    bmc.Checker
		wantErr bool
	}{
		"nothing given": {
			args: []string{"-checks", "c.py", "-evaluator-module", "g"},
			want: bmc.Checker{Checks: "c.py", EvaluatorModule: "g"},
		},
		"a function": {
			args: []string{"-checks", "c.py", "-evaluator-module", "g", "-env", "f=true"},
			want: bmc.Checker{Checks: "c.py", EvaluatorModule: "g", Env: []bmc.Binding{{Name: "f", Value: true}}},
		},
		"a seed, which csdfrepld does not take": {
			args:    []string{"-checks", "c.py", "-evaluator-module", "g", "-seed", "3"},
			wantErr: true,
		},
		"a function without a checks module": {
			args:    []string{"-env", "f=true"},
			wantErr: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			spy := cli.SpyProcInout()

			// Execute
			opts, err := NewParseOptionsFunc()(tt.args, spy.New())

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Error("want an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, opts.Checker); diff != "" {
				t.Errorf("Checker mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
