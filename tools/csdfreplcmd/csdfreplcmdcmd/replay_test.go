package csdfreplcmdcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/google/go-cmp/cmp"
)

func TestParseReplaySendsTheEnvironmentOfTheReport(t *testing.T) {
	seven := 7
	tests := map[string]struct {
		report string
		want   *animation.Environment
	}{
		"a seeded run": {
			report: `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}],"environment":{"seed":7,"given":["f"],"decided":[]}}`,
			want:   &animation.Environment{Seed: &seven, Given: []string{"f"}},
		},
		"a report of no environment": {
			report: `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}]}`,
			want:   nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			path := filepath.Join(t.TempDir(), "findings.json")
			if err := os.WriteFile(path, []byte(tt.report), 0o644); err != nil {
				t.Fatal(err)
			}
			spy := cli.SpyProcInout()

			// Execute
			opts, err := parseReplay()([]string{path}, spy.New())

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, opts.req.Environment); diff != "" {
				t.Errorf("the request's environment mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
