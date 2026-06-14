package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidExamples(t *testing.T) {
	examplesDir := "../examples/valid"

	files, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("Failed to read examples directory: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".puml" {
			t.Run(file.Name(), func(t *testing.T) {
				filePath := filepath.Join(examplesDir, file.Name())
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read file %s: %v", filePath, err)
				}

				parser := NewParser(string(content))
				diagram, err := parser.Parse()

				if err != nil {
					t.Errorf("Parse error for %s: %v", file.Name(), err)
					return
				}

				if diagram == nil {
					t.Errorf("Parser returned nil diagram for %s", file.Name())
					return
				}

				// Verify basic structure
				if len(diagram.States) == 0 {
					t.Errorf("No states found in %s", file.Name())
				}

				// Verify start edge exists (required for Composable State Diagrams Format)
				if diagram.StartEdge.Dst == "" {
					t.Errorf("No start edge found in %s - required for interface parallel", file.Name())
				}
			})
		}
	}
}

func TestParseInvalidExamples(t *testing.T) {
	examplesDir := "../examples/invalid"

	files, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("Failed to read examples directory: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".puml" {
			t.Run(file.Name(), func(t *testing.T) {
				filePath := filepath.Join(examplesDir, file.Name())
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read file %s: %v", filePath, err)
				}

				parser := NewParser(string(content))
				diagram, err := parser.Parse()

				if err == nil {
					t.Errorf("Expected parse error for invalid file %s, but parsing succeeded", file.Name())
					return
				}

				if diagram != nil {
					t.Errorf("Expected nil diagram for invalid file %s, but got non-nil diagram", file.Name())
				}
			})
		}
	}
}

func TestParseEndEdge(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantSrc   StateID
		wantGuard string
	}{
		{
			name: "with guard",
			input: `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*] : true
@enduml
`,
			wantSrc:   StateID("s0"),
			wantGuard: "true",
		},
		{
			name: "without guard",
			input: `@startuml
state "Done" as done
[*] --> done
done --> [*]
@enduml
`,
			wantSrc: StateID("done"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			parser := NewParser(tt.input)

			// Execute
			diagram, err := parser.Parse()

			// Assert
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if diagram.EndEdge == nil {
				t.Fatal("Parse() EndEdge = nil")
			}
			if diagram.EndEdge.Src != tt.wantSrc {
				t.Errorf("Parse() EndEdge.Src = %q, want %q", diagram.EndEdge.Src, tt.wantSrc)
			}
			if diagram.EndEdge.Guard != tt.wantGuard {
				t.Errorf("Parse() EndEdge.Guard = %q, want %q", diagram.EndEdge.Guard, tt.wantGuard)
			}

			// Teardown: no resources to release.
		})
	}
}
