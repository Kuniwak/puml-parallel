package csdfreplcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/google/go-cmp/cmp"
)

const givenReport = `{"deadlocks": [{"replay": {"initial": [0], "steps": []}}], "environment": {"given": ["f"], "env": {"f": false}, "decided": []}}`

func TestLoadReplayWalksTheEnvironmentOfTheReport(t *testing.T) {
	glue := bmc.Checker{Checks: "c.py", EvaluatorModule: "g"}
	with := func(env ...bmc.Binding) bmc.Checker {
		c := glue
		c.Env = env
		return c
	}
	f := bmc.Binding{Name: "f", Value: false}
	tests := map[string]struct {
		report  bool
		checker bmc.Checker
		want    bmc.Checker
		wantErr string
	}{
		"the same given function": {report: true, checker: with(f), want: with(f)},
		"the report's function":   {report: true, checker: glue, want: with(f)},
		"another value": {
			report: true, checker: with(bmc.Binding{Name: "f", Value: true}), wantErr: `the report's run gave "f" false, not true`,
		},
		"a function the run did not give": {
			report: true, checker: with(bmc.Binding{Name: "g", Value: true}), wantErr: `the checker gives "g" true`,
		},
		"no checker, which asks nothing":     {report: true, checker: bmc.Checker{}, want: bmc.Checker{}},
		"no report, which keeps the checker": {checker: with(f), want: with(f)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			opts := &Options{Checker: tt.checker}
			if tt.report {
				opts.Replay = filepath.Join(t.TempDir(), "findings.json")
				if err := os.WriteFile(opts.Replay, []byte(givenReport), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// Execute
			replay, got, err := loadReplay(opts)

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("loadReplay() error = %v, want one saying %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.report != (replay != nil) || (replay != nil && string(replay.Initial) != "[0]") {
				t.Errorf("loadReplay() replay = %+v, want the deadlock's, if any", replay)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("loadReplay() checker mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
