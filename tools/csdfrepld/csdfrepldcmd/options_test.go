package csdfrepldcmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/google/go-cmp/cmp"
)

func seed(n int) *int { return &n }

func TestNewParseOptionsFuncTakesTheEnvironment(t *testing.T) {
	tests := map[string]struct {
		args []string
		want bmc.Checker
	}{
		"nothing given": {
			args: []string{"-checks", "c.py", "-evaluator-module", "g"},
			want: bmc.Checker{Checks: "c.py", EvaluatorModule: "g"},
		},
		"a seed and a function": {
			args: []string{"-checks", "c.py", "-evaluator-module", "g", "-seed", "3", "-env", "f=true"},
			want: bmc.Checker{Checks: "c.py", EvaluatorModule: "g", Seed: seed(3), Env: []bmc.Binding{{Name: "f", Value: true}}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			spy := cli.SpyProcInout()

			// Execute
			opts, err := NewParseOptionsFunc()(tt.args, spy.New())

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, opts.Checker); diff != "" {
				t.Errorf("Checker mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewParseOptionsFuncRefusesAnEnvironmentWithoutChecks(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()

	// Execute
	_, err := NewParseOptionsFunc()([]string{"-env", "f=true"}, spy.New())

	// Assert
	if err == nil {
		t.Error("want an error, got nil")
	}
}
