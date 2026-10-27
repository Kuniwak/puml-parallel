package csdfreplcmdcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/google/go-cmp/cmp"
)

func TestParseReplaySendsTheEnvironmentOfTheReport(t *testing.T) {
	tests := map[string]struct {
		report  string
		want    *animation.Environment
		wantErr string
	}{
		"a run given a function": {
			report: `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}],"environment":{"given":["f"],"env":{"f":true},"decided":[]}}`,
			want:   &animation.Environment{Given: []string{"f"}, Env: map[string]bool{"f": true}},
		},
		"a report of no environment": {
			report: `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}]}`,
			want:   nil,
		},
		"a report of no finding": {
			report:  `{"deadlocks":[],"environment":{"given":[],"env":{},"decided":[]}}`,
			wantErr: "records no finding",
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
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseReplay() error = %v, want one saying %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, opts.req.Environment); diff != "" {
				t.Errorf("the request's environment mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
