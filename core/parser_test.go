package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidExamples(t *testing.T) {
	es1, err := os.ReadDir("../examples/valid")
	if err != nil {
		t.Fatalf("Failed to read examples directory: %v", err)
	}
	ps1 := make([]string, len(es1))
	for _, e := range es1 {
		ps1 = append(ps1, filepath.Join("../examples/valid", e.Name()))
	}

	es2, err := os.ReadDir("./testdata")
	if err != nil {
		t.Fatalf("Failed to read testdata directory: %v", err)
	}
	ps2 := make([]string, len(es2))
	for _, e := range es2 {
		ps2 = append(ps2, filepath.Join("./testdata", e.Name()))
	}

	ps := append(ps1, ps2...)

	for _, p := range ps {
		if filepath.Ext(p) != ".puml" {
			continue
		}
		t.Run(p, func(t *testing.T) {
			content, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", p, err)
			}

			parser := NewParser(string(content))
			diagram, err := parser.Parse()

			if err != nil {
				t.Errorf("Parse error for %s: %v", p, err)
				return
			}

			if diagram == nil {
				t.Errorf("Parser returned nil diagram for %s", p)
				return
			}
		})
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
