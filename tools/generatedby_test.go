package tools

import (
	"testing"
)

func TestGeneratedBy(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "no arguments",
			command: "csdfparallel",
			args:    nil,
			want:    "auto-generated-by: csdfparallel",
		},
		{
			name:    "plain arguments",
			command: "csdfparallel",
			args:    []string{"a.puml", "b.puml"},
			want:    "auto-generated-by: csdfparallel a.puml b.puml",
		},
		{
			name:    "argument needing quotes",
			command: "csdfparallel",
			args:    []string{"-sync", "insert;choose", "a.puml"},
			want:    "auto-generated-by: csdfparallel -sync 'insert;choose' a.puml",
		},
		{
			name:    "argument containing a single quote",
			command: "csdfcomp",
			args:    []string{"-base", "it's here", "-"},
			want:    `auto-generated-by: csdfcomp -base 'it'\''s here' -`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: no fixture to build.

			// Execute
			got := GeneratedBy(tt.command, tt.args)

			// Assert
			if got != tt.want {
				t.Errorf("GeneratedBy() = %q, want %q", got, tt.want)
			}

			// Teardown: no resources to release.
		})
	}
}
