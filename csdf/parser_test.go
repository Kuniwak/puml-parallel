package csdf

import (
	"os"
	"path/filepath"
	"strings"
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

func TestParseRejectsContentAfterEndEdge(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "duplicate end edge",
			input: `@startuml
state "Done" as done
[*] --> done
done --> [*]
done --> [*]
@enduml
`,
		},
		{
			name: "regular edge after end edge",
			input: `@startuml
state "Done" as done
[*] --> done
done --> [*]
done --> done : retry
@enduml
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			parser := NewParser(tt.input)

			// Execute
			diagram, err := parser.Parse()

			// Assert
			if err == nil {
				t.Fatal("Parse() error = nil, want content-after-end-edge rejection")
			}
			if diagram != nil {
				t.Errorf("Parse() diagram = %#v, want nil", diagram)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseRejectsSemicolonInEndEdgeGuard(t *testing.T) {
	// Setup
	parser := NewParser(`@startuml
state "Done" as done
[*] --> done
done --> [*] : left ; right
@enduml
`)

	// Execute
	diagram, err := parser.Parse()

	// Assert
	if err == nil {
		t.Fatal("Parse() error = nil, want semicolon rejection")
	}
	if diagram != nil {
		t.Errorf("Parse() diagram = %#v, want nil", diagram)
	}

	// Teardown: no resources to release.
}

func TestParseCommentsAndTypedStateVars(t *testing.T) {
	// Setup
	parser := NewParser(`@startuml /' diagram comment '/ "Example /' name '/"
' before first state
state /' before name '/ "Initial /' literal '/ and ' apostrophe" /' before as '/ as /' before id '/ s0 /' after id '/
' before first variable
s0 /' before colon '/ : /' before variable '/ ready /' before type marker '/ ; /' before type '/ bool /' after type '/
/' between variables '/
s0: cache ; map[string]/' inside type '/value
s0: optional ;
' before next state
state "Done" as s1
' before start edge
[*] /' before arrow '/ --> /' before destination '/ s0 /' before colon '/ : /' before post '/ initialize/' inside post '/now
' before regular edge
s0 /' before arrow '/ --> /' before destination '/ s1 /' before colon '/ : /' before event '/ finish(/' before parameter '/result/' before comma '/, /' before parameter '/code/' before close '/) /' before guard '/ ; /' before guard text '/ ready/' inside guard '/&& done /' before post '/ ; /' before post text '/ complete/' inside post '/now
' before end edge
s1 /' before arrow '/ --> /' before destination '/ [*] /' before colon '/ : /' before guard '/ "guard /' literal '/"
' comments are allowed after the end edge
/' final block comment '/
@enduml
`)

	// Execute
	diagram, err := parser.Parse()

	// Assert
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	initial := diagram.States["s0"]
	if initial.Name != "Initial /' literal '/ and ' apostrophe" {
		t.Errorf("Parse() state name = %q", initial.Name)
	}
	wantVars := []StateVar{
		{Name: "ready", Type: "bool"},
		{Name: "cache", Type: "map[string] value"},
		{Name: "optional"},
	}
	if len(initial.Vars) != len(wantVars) {
		t.Fatalf("Parse() vars = %#v, want %#v", initial.Vars, wantVars)
	}
	for i, want := range wantVars {
		if initial.Vars[i] != want {
			t.Errorf("Parse() vars[%d] = %#v, want %#v", i, initial.Vars[i], want)
		}
	}
	if diagram.StartEdge.Post != "initialize now" {
		t.Errorf("Parse() start post = %q, want %q", diagram.StartEdge.Post, "initialize now")
	}
	if len(diagram.Edges) != 1 {
		t.Fatalf("Parse() edges = %#v, want one edge", diagram.Edges)
	}
	edge := diagram.Edges[0]
	if edge.Event != "finish( result , code )" {
		t.Errorf("Parse() event = %q, want %q", edge.Event, "finish( result , code )")
	}
	if edge.Guard != "ready && done" {
		t.Errorf("Parse() guard = %q, want %q", edge.Guard, "ready && done")
	}
	if edge.Post != "complete now" {
		t.Errorf("Parse() post = %q, want %q", edge.Post, "complete now")
	}
	if diagram.EndEdge == nil {
		t.Fatal("Parse() end edge = nil")
	}
	if diagram.EndEdge.Guard != `"guard /' literal '/"` {
		t.Errorf("Parse() end guard = %q", diagram.EndEdge.Guard)
	}

	// Teardown: no resources to release.
}

func TestParseFreeFormEvents(t *testing.T) {
	tests := []struct {
		name      string
		event     string
		wantEvent Event
	}{
		{
			name:      "spaces symbols and unicode",
			event:     "注文 accepted => v2",
			wantEvent: "注文 accepted => v2",
		},
		{
			name:      "unclosed parenthesis",
			event:     "finish(result",
			wantEvent: "finish(result",
		},
		{
			name:      "trailing comma",
			event:     "finish(result, )",
			wantEvent: "finish(result, )",
		},
		{
			name:      "block comment removed",
			event:     "send/' implementation detail '/message",
			wantEvent: "send message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : ` + tt.event + ` ; ready ; done
@enduml
`)

			diagram, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(diagram.Edges) != 1 {
				t.Fatalf("Parse() edges = %#v, want one edge", diagram.Edges)
			}
			if diagram.Edges[0].Event != tt.wantEvent {
				t.Errorf("Parse() event = %q, want %q", diagram.Edges[0].Event, tt.wantEvent)
			}
			if diagram.Edges[0].Guard != "ready" {
				t.Errorf("Parse() guard = %q, want ready", diagram.Edges[0].Guard)
			}
			if diagram.Edges[0].Post != "done" {
				t.Errorf("Parse() post = %q, want done", diagram.Edges[0].Post)
			}
		})
	}
}

func TestParseRejectsEmptyEvent(t *testing.T) {
	tests := []struct {
		name  string
		event string
	}{
		{name: "empty", event: ""},
		{name: "whitespace", event: " \t "},
		{name: "comment only", event: " /' comment '/ "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : ` + tt.event + `
@enduml
`)

			diagram, err := parser.Parse()
			if err == nil {
				t.Fatal("Parse() error = nil, want empty event rejection")
			}
			if diagram != nil {
				t.Errorf("Parse() diagram = %#v, want nil", diagram)
			}
		})
	}
}

func TestParseIgnoreRegion(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "directives wrapped in markers",
			input: `@startuml
' CSDF-IGNORE-BEGIN
left to right direction
skinparam backgroundColor #EEEBDC
' CSDF-IGNORE-END
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : finish
@enduml
`,
		},
		{
			name: "indented markers",
			input: `@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
    ' CSDF-IGNORE-BEGIN
    left to right direction
    ' CSDF-IGNORE-END
s0 --> s1 : finish
@enduml
`,
		},
		{
			name: "no space after apostrophe",
			input: `@startuml
'CSDF-IGNORE-BEGIN
left to right direction
'CSDF-IGNORE-END
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : finish
@enduml
`,
		},
		{
			// Each region closes at its first CSDF-IGNORE-END (not greedy to the
			// last one); the state declared between the two regions must survive.
			name: "two regions close at first end each",
			input: `@startuml
' CSDF-IGNORE-BEGIN
left to right direction
' CSDF-IGNORE-END
state "Initial" as s0
' CSDF-IGNORE-BEGIN
skinparam backgroundColor #EEEBDC
' CSDF-IGNORE-END
state "Done" as s1
[*] --> s0
s0 --> s1 : finish
@enduml
`,
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
			if len(diagram.States) != 2 {
				t.Errorf("Parse() states = %#v, want two states", diagram.States)
			}
			if diagram.StartEdge.Dst != "s0" {
				t.Errorf("Parse() start edge dst = %q, want s0", diagram.StartEdge.Dst)
			}
			if len(diagram.Edges) != 1 {
				t.Fatalf("Parse() edges = %#v, want one edge", diagram.Edges)
			}
			if diagram.Edges[0].Event != "finish" {
				t.Errorf("Parse() event = %q, want finish", diagram.Edges[0].Event)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseRejectsUnterminatedIgnoreRegion(t *testing.T) {
	// Setup
	parser := NewParser(`@startuml
' CSDF-IGNORE-BEGIN
left to right direction
@enduml
`)

	// Execute
	diagram, err := parser.Parse()

	// Assert
	if err == nil {
		t.Fatal("Parse() error = nil, want unterminated CSDF-IGNORE region error")
	}
	if diagram != nil {
		t.Errorf("Parse() diagram = %#v, want nil", diagram)
	}
	if !strings.Contains(err.Error(), "unterminated CSDF-IGNORE region at line 2, col 1") {
		t.Errorf("Parse() error = %q, want unterminated region at line 2, col 1", err)
	}

	// Teardown: no resources to release.
}

func TestParseRejectsUnterminatedBlockComment(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantPos string
	}{
		{
			name: "between declarations",
			input: `@startuml
/' never closed
`,
			wantPos: "line 2, col 1",
		},
		{
			name: "inside declaration",
			input: `@startuml
state /' never closed
`,
			wantPos: "line 2, col 7",
		},
		{
			name: "inside free text",
			input: `@startuml
state "Initial" as s0
[*] --> s0
s0 --> s0 : retry ; ready /' never closed
`,
			wantPos: "line 4, col 27",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			parser := NewParser(tt.input)

			// Execute
			diagram, err := parser.Parse()

			// Assert
			if err == nil {
				t.Fatal("Parse() error = nil, want unterminated block comment error")
			}
			if diagram != nil {
				t.Errorf("Parse() diagram = %#v, want nil", diagram)
			}
			if !strings.Contains(err.Error(), "unterminated block comment at "+tt.wantPos) {
				t.Errorf("Parse() error = %q, want block comment start at %s", err, tt.wantPos)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseRejectsSemicolonInStateVarType(t *testing.T) {
	// Setup
	parser := NewParser(`@startuml
state "Initial" as s0
s0: ready ; bool ; extra
[*] --> s0
@enduml
`)

	// Execute
	diagram, err := parser.Parse()

	// Assert
	if err == nil {
		t.Fatal("Parse() error = nil, want semicolon rejection")
	}
	if diagram != nil {
		t.Errorf("Parse() diagram = %#v, want nil", diagram)
	}

	// Teardown: no resources to release.
}
