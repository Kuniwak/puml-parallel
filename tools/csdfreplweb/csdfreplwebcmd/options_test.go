package csdfreplwebcmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFunc(t *testing.T) {
	tests := map[string]struct {
		args       []string
		wantFile   string
		wantExec   []string
		wantEngine string
		wantErr    bool
	}{
		"a diagram": {
			args:     []string{"d.puml"},
			wantFile: "d.puml",
			wantExec: []string{"plantuml", "-pipe", "-tsvg"},
		},
		"a PlantUML command quoting a path": {
			args:     []string{"-puml-exec", `java -jar "my dir/plantuml.jar" -pipe -tsvg`, "d.puml"},
			wantFile: "d.puml",
			wantExec: []string{"java", "-jar", "my dir/plantuml.jar", "-pipe", "-tsvg"},
		},
		"an engine": {
			args:       []string{"-engine", "e.wasm.gz", "d.puml"},
			wantFile:   "d.puml",
			wantExec:   []string{"plantuml", "-pipe", "-tsvg"},
			wantEngine: "e.wasm.gz",
		},
		"a PlantUML command whose quote is not closed": {
			args:    []string{"-puml-exec", `java "x`, "d.puml"},
			wantErr: true,
		},
		"no diagram": {
			args:    []string{},
			wantErr: true,
		},
		"two diagrams": {
			args:    []string{"a.puml", "b.puml"},
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
			if opts.File != tt.wantFile {
				t.Errorf("File = %q, want %q", opts.File, tt.wantFile)
			}
			if opts.Engine != tt.wantEngine {
				t.Errorf("Engine = %q, want %q", opts.Engine, tt.wantEngine)
			}
			if diff := cmp.Diff(tt.wantExec, opts.PumlExec); diff != "" {
				t.Errorf("PumlExec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewParseOptionsFuncTakesHelp(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()

	// Execute
	opts, err := NewParseOptionsFunc()([]string{"-h"}, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Common.Help {
		t.Error("Help = false, want true")
	}
}
