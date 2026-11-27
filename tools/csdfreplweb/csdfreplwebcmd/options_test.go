package csdfreplwebcmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/v9/cli"
	"github.com/Kuniwak/puml-parallel/v9/csdf/animation/bmc"
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

func TestNewParseOptionsFuncTakesTheChecker(t *testing.T) {
	tests := map[string]struct {
		args        []string
		wantChecker bmc.Checker
		wantPyodide string
		wantErr     bool
	}{
		"no checker": {
			args:        []string{"d.puml"},
			wantPyodide: DefaultPyodide,
		},
		"a checker with a glue module and the environment": {
			args: []string{"-checks", "c.py", "-evaluator-module", "csdfbmc.python_glue c.py", "-env", "$_0 ok=true",
				"-pyodide", "https://example.com/pyodide/", "d.puml"},
			wantChecker: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue c.py", Env: []bmc.Binding{{Name: "$_0 ok", Value: true}}},
			wantPyodide: "https://example.com/pyodide/",
		},
		"a glue command, which a browser cannot start": {
			args:    []string{"-checks", "c.py", "-evaluator", "python3 -m csdfbmc.python_glue", "d.puml"},
			wantErr: true,
		},
		"a checks module without a glue": {
			args:    []string{"-checks", "c.py", "d.puml"},
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
			if diff := cmp.Diff(tt.wantChecker, opts.Checker); diff != "" {
				t.Errorf("Checker mismatch (-want +got):\n%s", diff)
			}
			if opts.Pyodide != tt.wantPyodide {
				t.Errorf("Pyodide = %q, want %q", opts.Pyodide, tt.wantPyodide)
			}
		})
	}
}

func TestNewParseOptionsFuncTakesTheShapes(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()

	// Execute
	opts, err := NewParseOptionsFunc()([]string{"-shapes", "s.json", "d.puml"}, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if opts.Shapes != "s.json" {
		t.Errorf("Shapes = %q, want s.json", opts.Shapes)
	}
}

func TestNewParseOptionsFuncTakesTheProse(t *testing.T) {
	// Setup
	spy := cli.SpyProcInout()

	// Execute
	opts, err := NewParseOptionsFunc()([]string{"-prose", "en=p.puml", "-prose", "ja=p_en.puml", "d.puml"}, spy.New())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]string{"en": "p.puml", "ja": "p_en.puml"}, opts.Prose); diff != "" {
		t.Errorf("Prose mismatch (-want +got):\n%s", diff)
	}
}

func TestNewParseOptionsFuncRefusesAProseOfNoKnownLanguage(t *testing.T) {
	tests := map[string][]string{
		"no language":      {"-prose", "p_ja.puml", "d.puml"},
		"unknown language": {"-prose", "fr=p.puml", "d.puml"},
		"no file":          {"-prose", "ja=", "d.puml"},
		"language twice":   {"-prose", "ja=p.puml", "-prose", "ja=q.puml", "d.puml"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			spy := cli.SpyProcInout()

			// Execute
			_, err := NewParseOptionsFunc()(args, spy.New())

			// Assert
			if err == nil {
				t.Errorf("NewParseOptionsFunc(%q) error = nil, want one", args)
			}
		})
	}
}
