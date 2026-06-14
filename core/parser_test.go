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

func TestParseEndEdgeWithGuard(t *testing.T) {
	input := `@startuml
state "SKIP" as s0
[*] --> s0
s0 --> [*] : true
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if diagram.EndEdge == nil {
		t.Fatal("Parse() EndEdge = nil")
	}
	if diagram.EndEdge.Src != StateID("s0") {
		t.Errorf("Parse() EndEdge.Src = %q, want %q", diagram.EndEdge.Src, StateID("s0"))
	}
	if diagram.EndEdge.Guard != "true" {
		t.Errorf("Parse() EndEdge.Guard = %q, want %q", diagram.EndEdge.Guard, "true")
	}
}

func TestParseEndEdgeWithoutGuard(t *testing.T) {
	input := `@startuml
state "Done" as done
[*] --> done
done --> [*]
@enduml
`

	diagram, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if diagram.EndEdge == nil {
		t.Fatal("Parse() EndEdge = nil")
	}
	if diagram.EndEdge.Src != StateID("done") {
		t.Errorf("Parse() EndEdge.Src = %q, want %q", diagram.EndEdge.Src, StateID("done"))
	}
	if diagram.EndEdge.Guard != "" {
		t.Errorf("Parse() EndEdge.Guard = %q, want empty", diagram.EndEdge.Guard)
	}
}
