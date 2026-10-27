package csdf

import (
	"os"
	"path/filepath"
	"slices"
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

				parser := newParser(string(content))
				diagram, err := parser.parse()

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

				parser := newParser(string(content))
				diagram, err := parser.parse()

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
		wantLine  int
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
			wantLine:  4,
		},
		{
			name: "without guard",
			input: `@startuml
state "Done" as done
[*] --> done
done --> [*]
@enduml
`,
			wantSrc:  StateID("done"),
			wantLine: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			parser := newParser(tt.input)

			// Execute
			diagram, err := parser.parse()

			// Assert
			if err != nil {
				t.Fatalf("parseAccepting() error = %v", err)
			}
			if diagram.EndEdge == nil {
				t.Fatal("parseAccepting() EndEdge = nil")
			}
			if diagram.EndEdge.Src != tt.wantSrc {
				t.Errorf("parseAccepting() EndEdge.Src = %q, want %q", diagram.EndEdge.Src, tt.wantSrc)
			}
			if diagram.EndEdge.Guard != Predicate(tt.wantGuard) {
				t.Errorf("parseAccepting() EndEdge.Guard = %q, want %q", diagram.EndEdge.Guard, tt.wantGuard)
			}
			// The line the declaration starts on, like every other edge: it names
			// the end edge's generated predicate aliases downstream.
			if diagram.EndEdge.Line != tt.wantLine {
				t.Errorf("parseAccepting() EndEdge.Line = %d, want %d", diagram.EndEdge.Line, tt.wantLine)
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
			parser := newParser(tt.input)

			// Execute
			diagram, err := parser.parse()

			// Assert
			if err == nil {
				t.Fatal("parseAccepting() error = nil, want content-after-end-edge rejection")
			}
			if diagram != nil {
				t.Errorf("parseAccepting() diagram = %#v, want nil", diagram)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseRejectsSemicolonInEndEdgeGuard(t *testing.T) {
	// Setup
	parser := newParser(`@startuml
state "Done" as done
[*] --> done
done --> [*] : left ; right
@enduml
`)

	// Execute
	diagram, err := parser.parse()

	// Assert
	if err == nil {
		t.Fatal("parseAccepting() error = nil, want semicolon rejection")
	}
	if diagram != nil {
		t.Errorf("parseAccepting() diagram = %#v, want nil", diagram)
	}

	// Teardown: no resources to release.
}

func TestParseCommentsAndTypedStateVars(t *testing.T) {
	// Setup
	parser := newParser(`@startuml /' diagram comment '/ "Example /' name '/"
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
s0 /' before arrow '/ --> /' before destination '/ s1 /' before colon '/ : /' before event '/ finish/' before field '/?result/' between fields '/?code /' before guard '/ ; /' before guard text '/ ready/' inside guard '/&& done /' before post '/ ; /' before post text '/ complete/' inside post '/now
' before end edge
s1 /' before arrow '/ --> /' before destination '/ [*] /' before colon '/ : /' before guard '/ "guard /' literal '/"
' comments are allowed after the end edge
/' final block comment '/
@enduml
`)

	// Execute
	diagram, err := parser.parse()

	// Assert
	if err != nil {
		t.Fatalf("parseAccepting() error = %v", err)
	}
	initial := diagram.States["s0"]
	if initial.Name != "Initial /' literal '/ and ' apostrophe" {
		t.Errorf("parseAccepting() state name = %q", initial.Name)
	}
	wantVars := []StateVar{
		{Name: "ready", Type: "bool"},
		{Name: "cache", Type: "map[string] value"},
		{Name: "optional"},
	}
	if len(initial.Vars) != len(wantVars) {
		t.Fatalf("parseAccepting() vars = %#v, want %#v", initial.Vars, wantVars)
	}
	for i, want := range wantVars {
		if initial.Vars[i] != want {
			t.Errorf("parseAccepting() vars[%d] = %#v, want %#v", i, initial.Vars[i], want)
		}
	}
	if diagram.StartEdge.Post != "initialize now" {
		t.Errorf("parseAccepting() start post = %q, want %q", diagram.StartEdge.Post, "initialize now")
	}
	if len(diagram.Edges) != 1 {
		t.Fatalf("parseAccepting() edges = %#v, want one edge", diagram.Edges)
	}
	edge := diagram.Edges[0]
	if edge.Event != "finish?result?code" {
		t.Errorf("parseAccepting() event = %q, want %q", edge.Event, "finish?result?code")
	}
	if edge.Guard != "ready && done" {
		t.Errorf("parseAccepting() guard = %q, want %q", edge.Guard, "ready && done")
	}
	if edge.Post != "complete now" {
		t.Errorf("parseAccepting() post = %q, want %q", edge.Post, "complete now")
	}
	if diagram.EndEdge == nil {
		t.Fatal("parseAccepting() end edge = nil")
	}
	// A clause language's quotes are not the tools': the comment is removed.
	if diagram.EndEdge.Guard != `"guard "` {
		t.Errorf("parseAccepting() end guard = %q", diagram.EndEdge.Guard)
	}

	// Teardown: no resources to release.
}

func TestParseRecordsEdgeLine(t *testing.T) {
	// Setup: each edge sits on a known source line.
	parser := newParser(`@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : e1
@enduml
`)

	// Execute
	diagram, err := parser.parse()

	// Assert
	if err != nil {
		t.Fatalf("parseAccepting() error = %v", err)
	}
	if diagram.StartEdge.Line != 4 {
		t.Errorf("parseAccepting() start edge line = %d, want 4", diagram.StartEdge.Line)
	}
	if len(diagram.Edges) != 2 {
		t.Fatalf("parseAccepting() edges = %#v, want two edges", diagram.Edges)
	}
	if diagram.Edges[0].Line != 5 {
		t.Errorf("parseAccepting() edges[0] line = %d, want 5", diagram.Edges[0].Line)
	}
	if diagram.Edges[1].Line != 6 {
		t.Errorf("parseAccepting() edges[1] line = %d, want 6", diagram.Edges[1].Line)
	}

	// Teardown: no resources to release.
}

func TestParseEvents(t *testing.T) {
	tests := []struct {
		name        string
		event       string
		wantEvent   Event
		wantChannel string
		wantFields  []Field
	}{
		{name: "a channel alone", event: "reset", wantEvent: "reset", wantChannel: "reset"},
		{name: "received and chosen fields", event: "c?x!y", wantEvent: "c?x!y", wantChannel: "c",
			wantFields: []Field{{Dir: Received, Name: "x"}, {Dir: Sent, Name: "y"}}},
		{name: "unicode, digits and snake case", event: "AUDIT_BEGIN?監査ID?item2", wantEvent: "AUDIT_BEGIN?監査ID?item2", wantChannel: "AUDIT_BEGIN",
			wantFields: []Field{{Dir: Received, Name: "監査ID"}, {Dir: Received, Name: "item2"}}},
		{name: "block comments and spaces between tokens removed", event: "send /' implementation detail '/ ? m", wantEvent: "send?m", wantChannel: "send",
			wantFields: []Field{{Dir: Received, Name: "m"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : ` + tt.event + ` ; ready ; done
@enduml
`)

			diagram, err := parser.parse()
			if err != nil {
				t.Fatalf("parseAccepting() error = %v", err)
			}
			e := diagram.Edges[0].Event
			if e != tt.wantEvent {
				t.Errorf("parseAccepting() event = %q, want %q", e, tt.wantEvent)
			}
			if e.Channel() != tt.wantChannel {
				t.Errorf("Channel() = %q, want %q", e.Channel(), tt.wantChannel)
			}
			if !slices.Equal(e.Fields(), tt.wantFields) {
				t.Errorf("Fields() = %#v, want %#v", e.Fields(), tt.wantFields)
			}
		})
	}
}

func TestParseRejectsEventsOutsideTheGrammar(t *testing.T) {
	for _, event := range []string{"finish(result)", "注文 accepted", "c?", "c?x?", "?x", "c!!y", "c x", "a.b"} {
		t.Run(event, func(t *testing.T) {
			parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : ` + event + `
@enduml
`)
			if _, err := parser.parse(); err == nil {
				t.Fatalf("parseAccepting() error = nil, want %q refused", event)
			}
		})
	}
}

func TestParseRefusesAnEventThatRepeatsAFieldName(t *testing.T) {
	for _, event := range []string{"c?x!x", "c?x?x", "c!x!x", "c?x?y!x"} {
		t.Run(event, func(t *testing.T) {
			parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : ` + event + `
@enduml
`)
			_, err := parser.parse()
			if err == nil || !strings.Contains(err.Error(), event) || !strings.Contains(err.Error(), "repeats the field x") {
				t.Fatalf("parse() error = %v, want %q refused for repeating the field x", err, event)
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
			parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : ` + tt.event + `
@enduml
`)

			diagram, err := parser.parse()
			if err == nil {
				t.Fatal("parseAccepting() error = nil, want empty event rejection")
			}
			if diagram != nil {
				t.Errorf("parseAccepting() diagram = %#v, want nil", diagram)
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
			parser := newParser(tt.input)

			// Execute
			diagram, err := parser.parse()

			// Assert
			if err != nil {
				t.Fatalf("parseAccepting() error = %v", err)
			}
			if len(diagram.States) != 2 {
				t.Errorf("parseAccepting() states = %#v, want two states", diagram.States)
			}
			if diagram.StartEdge.Dst != "s0" {
				t.Errorf("parseAccepting() start edge dst = %q, want s0", diagram.StartEdge.Dst)
			}
			if len(diagram.Edges) != 1 {
				t.Fatalf("parseAccepting() edges = %#v, want one edge", diagram.Edges)
			}
			if diagram.Edges[0].Event != "finish" {
				t.Errorf("parseAccepting() event = %q, want finish", diagram.Edges[0].Event)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseRejectsUnterminatedIgnoreRegion(t *testing.T) {
	// Setup
	parser := newParser(`@startuml
' CSDF-IGNORE-BEGIN
left to right direction
@enduml
`)

	// Execute
	diagram, err := parser.parse()

	// Assert
	if err == nil {
		t.Fatal("parseAccepting() error = nil, want unterminated CSDF-IGNORE region error")
	}
	if diagram != nil {
		t.Errorf("parseAccepting() diagram = %#v, want nil", diagram)
	}
	if !strings.Contains(err.Error(), "unterminated CSDF-IGNORE region at line 2, col 1") {
		t.Errorf("parseAccepting() error = %q, want unterminated region at line 2, col 1", err)
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
			parser := newParser(tt.input)

			// Execute
			diagram, err := parser.parse()

			// Assert
			if err == nil {
				t.Fatal("parseAccepting() error = nil, want unterminated block comment error")
			}
			if diagram != nil {
				t.Errorf("parseAccepting() diagram = %#v, want nil", diagram)
			}
			if !strings.Contains(err.Error(), "unterminated block comment at "+tt.wantPos) {
				t.Errorf("parseAccepting() error = %q, want block comment start at %s", err, tt.wantPos)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseRejectsSemicolonInStateVarType(t *testing.T) {
	// Setup
	parser := newParser(`@startuml
state "Initial" as s0
s0: ready ; bool ; extra
[*] --> s0
@enduml
`)

	// Execute
	diagram, err := parser.parse()

	// Assert
	if err == nil {
		t.Fatal("parseAccepting() error = nil, want semicolon rejection")
	}
	if diagram != nil {
		t.Errorf("parseAccepting() diagram = %#v, want nil", diagram)
	}

	// Teardown: no resources to release.
}

func TestParseDiagramName(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{name: "quoted", title: `"Vending Machine"`, want: `"Vending Machine"`},
		{name: "bare", title: "VendingMachine", want: "VendingMachine"},
		{name: "bare with spaces", title: "Vending Machine", want: "Vending Machine"},
		{name: "bare with extension", title: "vending-machine.png", want: "vending-machine.png"},
		{name: "trailing whitespace", title: "VendingMachine  ", want: "VendingMachine"},
		{name: "comment delimiters are plain text", title: "Vending /' Machine", want: "Vending /' Machine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			parser := newParser(`@startuml ` + tt.title + `
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : e
@enduml
`)

			// Execute
			diagram, err := parser.parse()

			// Assert
			if err != nil {
				t.Fatalf("parseAccepting() error = %v, want nil", err)
			}
			if len(diagram.States) != 2 {
				t.Errorf("parseAccepting() states = %#v, want two states", diagram.States)
			}
			if len(diagram.Edges) != 1 {
				t.Fatalf("parseAccepting() edges = %#v, want one edge", diagram.Edges)
			}
			if diagram.Name != tt.want {
				t.Errorf("parseAccepting().Name = %q, want %q", diagram.Name, tt.want)
			}

			// Teardown: no resources to release.
		})
	}
}

func TestParseWithoutDiagramName(t *testing.T) {
	// Setup
	parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : e
@enduml
`)

	// Execute
	diagram, err := parser.parse()

	// Assert
	if err != nil {
		t.Fatalf("parseAccepting() error = %v, want nil", err)
	}
	if diagram.Name != "" {
		t.Errorf("parseAccepting().Name = %q, want empty", diagram.Name)
	}

	// Teardown: no resources to release.
}

func TestParseUnicodeIdentifiers(t *testing.T) {
	input := `@startuml "ja"
[*] --> 約定済み
state "約定済み" as 約定済み
約定済み : 約定内容 ; 約定内容
約定済み --> [*]: 完了
@enduml
`

	diagram, err := newParser(input).parse()
	if err != nil {
		t.Fatalf("parseAccepting() error = %v, want nil", err)
	}

	state, ok := diagram.States["約定済み"]
	if !ok {
		t.Fatalf("diagram.States has no state 約定済み, got %v", diagram.States)
	}
	if len(state.Vars) != 1 || state.Vars[0].Name != "約定内容" {
		t.Errorf("state.Vars = %v, want [約定内容]", state.Vars)
	}
	if diagram.StartEdge.Dst != "約定済み" {
		t.Errorf("diagram.StartEdge.Dst = %q, want %q", diagram.StartEdge.Dst, "約定済み")
	}
}

func TestParseClausesWithTheToolsVocabulary(t *testing.T) {
	parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0 : @opaque
s0 --> s1 : go ; f(x) \; g(x) ∧ @opaque ; a \∧ b
s1 --> [*] : mail \@ host
@enduml
`)

	diagram, err := parser.parse()
	if err != nil {
		t.Fatalf("parseAccepting() error = %v", err)
	}
	if got, want := diagram.Edges[0].Guard, Predicate(`f(x) \; g(x) ∧ @opaque`); got != want {
		t.Errorf("guard = %q, want %q", got, want)
	}
	if got, want := diagram.Edges[0].Post, Predicate(`a \∧ b`); got != want {
		t.Errorf("post = %q, want %q", got, want)
	}
}

func TestParseTakesNoQuotesOfAClauseLanguage(t *testing.T) {
	parser := newParser(`@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : go ; s == "a;b" ; t == "a\;b" \\; u
@enduml
`)

	diagram, err := parser.parse()
	if err != nil {
		t.Fatalf("parseAccepting() error = %v", err)
	}
	if got, want := diagram.Edges[0].Guard, Predicate(`s == "a`); got != want {
		t.Errorf("guard = %q, want %q", got, want)
	}
	if got, want := diagram.Edges[0].Post, Predicate(`b" ; t == "a\;b" \\; u`); got != want {
		t.Errorf("post = %q, want %q", got, want)
	}
}
