package csdfreplcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/google/go-cmp/cmp"
)

const seededReport = `{"deadlocks": [{"replay": {"initial": [0], "steps": []}}], "environment": {"seed": 7, "given": ["f"], "decided": []}}`

func TestLoadReplayWalksTheEnvironmentOfTheReport(t *testing.T) {
	glue := bmc.Checker{Checks: "c.py", EvaluatorModule: "g"}
	with := func(s *int, env ...bmc.Binding) bmc.Checker {
		c := glue
		c.Seed, c.Env = s, env
		return c
	}
	f := bmc.Binding{Name: "f", Value: false}
	tests := map[string]struct {
		checker bmc.Checker
		want    bmc.Checker
		wantErr string
	}{
		"the report's seed":                  {checker: glue, want: with(seed(7))},
		"the same seed and a given function": {checker: with(seed(7), f), want: with(seed(7), f)},
		"another seed":                       {checker: with(seed(1)), wantErr: "not -seed 1"},
		"a function the run did not give": {
			checker: with(nil, bmc.Binding{Name: "g", Value: true}), wantErr: `-env gives "g"`,
		},
		"no checker, which asks nothing": {checker: bmc.Checker{}, want: bmc.Checker{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			path := filepath.Join(t.TempDir(), "findings.json")
			if err := os.WriteFile(path, []byte(seededReport), 0o644); err != nil {
				t.Fatal(err)
			}
			opts := &Options{Replay: path, Checker: tt.checker}

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
			if replay == nil || string(replay.Initial) != "[0]" {
				t.Errorf("loadReplay() replay = %+v, want the deadlock's", replay)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("loadReplay() checker mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadReplayWithoutAReportKeepsTheChecker(t *testing.T) {
	// Setup
	checker := bmc.Checker{Checks: "c.py", EvaluatorModule: "g", Seed: seed(1)}

	// Execute
	replay, got, err := loadReplay(&Options{Checker: checker})

	// Assert
	if err != nil || replay != nil {
		t.Fatalf("loadReplay() = %v, %v; want no replay", replay, err)
	}
	if diff := cmp.Diff(checker, got); diff != "" {
		t.Errorf("loadReplay() checker mismatch (-want +got):\n%s", diff)
	}
}
