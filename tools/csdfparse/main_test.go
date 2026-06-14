package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
	}{
		{
			name: "prints start edge",
			input: `@startuml
state "Start" as s0
[*] --> s0
@enduml
`,
			wantOutput: "Start Edge:\n  [*] --> s0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			// Execute
			exitCode := Run(strings.NewReader(tt.input), stdout, stderr)

			// Assert
			if exitCode != 0 {
				t.Errorf("Run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantOutput) {
				t.Errorf("Run() stdout = %q, want substring %q", stdout.String(), tt.wantOutput)
			}
			if stderr.Len() != 0 {
				t.Errorf("Run() stderr = %q, want empty", stderr.String())
			}

			// Teardown: no resources to release.
		})
	}
}
