package core

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestParseValidExamples(t *testing.T) {
	examplesDir := "../examples/valid"
	
	files, err := ioutil.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("Failed to read examples directory: %v", err)
	}
	
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".puml" {
			t.Run(file.Name(), func(t *testing.T) {
				filePath := filepath.Join(examplesDir, file.Name())
				content, err := ioutil.ReadFile(filePath)
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
					t.Errorf("No start edge found in %s - required for parallel composition", file.Name())
				}
			})
		}
	}
}

func TestParseInvalidExamples(t *testing.T) {
	examplesDir := "../examples/invalid"
	
	files, err := ioutil.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("Failed to read examples directory: %v", err)
	}
	
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".puml" {
			t.Run(file.Name(), func(t *testing.T) {
				filePath := filepath.Join(examplesDir, file.Name())
				content, err := ioutil.ReadFile(filePath)
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