package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Kuniwak/puml-parallel/core"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestEventDisplaysGolden(t *testing.T) {
	state := RuntimeState{
		ID:   "review",
		Name: "Review order",
		Values: []StateValue{
			{Name: "count", Value: 2},
			{Name: "metadata", Value: map[string]any{"priority": true, "tags": []any{"new", "gift"}}},
		},
	}
	history := []HistoryEntry{
		{
			State: RuntimeState{
				ID:     "draft",
				Name:   "Draft order",
				Values: []StateValue{{Name: "count", Value: 1}},
			},
			Trace: []core.Event{},
		},
		{
			State: state,
			Trace: []core.Event{"submit(order)", "approve"},
		},
	}

	tests := []struct {
		name    string
		diagram *core.Diagram
		display func(*repl)
	}{
		{
			name: "EventDisplayFatal",
			display: func(r *repl) {
				r.displayFatal(`destination state "missing" does not exist`)
			},
		},
		{
			name: "EventDisplayStateGroup",
			display: func(r *repl) {
				r.displayStateValuePrompt(core.State{
					ID:   "review",
					Name: "Review order",
					Vars: []core.StateVar{
						{Name: "count", Type: "number"},
						{Name: "metadata"},
					},
				}, "count > 0", `status' = "reviewing"`)
			},
		},
		{
			name: "EventDisplayError",
			display: func(r *repl) {
				r.displayError("State variable values length mismatch")
			},
		},
		{
			name: "EventDisplayTrans",
			diagram: &core.Diagram{
				States: map[core.StateID]core.State{
					"approved": {ID: "approved", Name: "Approved"},
					"rejected": {ID: "rejected", Name: "Rejected"},
				},
				Edges: []core.Edge{
					{
						Src:   "review",
						Dst:   "approved",
						Event: "approve",
						Guard: "count > 0",
						Post:  `status' = "approved"`,
					},
					{
						Src:   "review",
						Dst:   "rejected",
						Event: "reject(reason)",
					},
				},
			},
			display: func(r *repl) {
				r.displayState(state)
			},
		},
		{
			name:    "EventDisplayDeadlock",
			diagram: &core.Diagram{},
			display: func(r *repl) {
				r.displayState(state)
			},
		},
		{
			name: "EventDisplayTrace",
			display: func(r *repl) {
				r.displayJSON("Trace", []core.Event{"submit(order)", "approve"})
			},
		},
		{
			name: "EventDisplayHistory",
			display: func(r *repl) {
				r.displayJSON("History", history)
			},
		},
		{
			name: "EventDisplayHelp",
			display: func(r *repl) {
				r.displayHelp()
			},
		},
		{
			name: "EventDisplayEmptyLine",
			display: func(r *repl) {
				r.displayEmptyLine()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			diagram := tt.diagram
			if diagram == nil {
				diagram = &core.Diagram{}
			}
			tt.display(&repl{diagram: diagram, stdout: stdout})
			assertGolden(t, tt.name, stdout.Bytes())
		})
	}
}

func assertGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file: %v; create it with go test ./tools/csdfrepl -update", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Errorf("output differs from %s\n--- expected ---\n%s--- actual ---\n%s\nupdate with: go test ./tools/csdfrepl -update", path, expected, actual)
	}
}

func TestRunExploresDiagram(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
s0: count ; number
state "Done" as s1
s1: result ; string
[*] --> s0 : count starts at zero
s0 --> s1 : insert(coin) ; count >= 0 ; result is done
@enduml
`)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	input := strings.NewReader("[0]\nl\nt\nh\ns 0\n[\"ok\"]\nt\nj 0\n\n")

	exitCode := Run([]string{path}, input, stdout, stderr, nil)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("Run() stderr = %q, want empty", stderr.String())
	}
	for _, want := range []string{
		"State: Initial (s0)",
		"count: number",
		"[0] insert(coin) -> Done (s1)",
		`"insert(coin)"`,
		"State: Done (s1)",
		`result = "ok"`,
		"Deadlock: no outgoing transitions.",
		"History:",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("Run() stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunReportsInputErrorsAndRecovers(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
s0: value
[*] --> s0
@enduml
`)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	input := strings.NewReader("null\n{}\n[]\n[null]\n[1]\ns\ns nope\ns 0\nj\nj nope\nj 9\nl extra\nunknown\n")

	exitCode := Run([]string{path}, input, stdout, stderr, nil)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	for _, want := range []string{
		"invalid JSON array",
		"State variable values length mismatch",
		"null is not a supported JSON value",
		"required an index of transition",
		"Not a natural number",
		"Index out of range",
		"required an index of history",
		"invalid arguments",
		"invalid command",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("Run() stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunKeepsNoSolutionsEscapeHatch(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
[*] --> s0
@enduml
`)
	stdout := &bytes.Buffer{}
	solver := &sequenceSolver{
		results: []PostSolverResult{
			{Kind: PostSolverResultNoSolutions},
			{
				Kind:  PostSolverResultOK,
				State: RuntimeState{ID: "s0", Name: "Initial"},
			},
		},
	}

	exitCode := runWithSolver([]string{path}, strings.NewReader("[]\n[]\n"), stdout, &bytes.Buffer{}, nil, solver)

	if exitCode != 0 {
		t.Fatalf("runWithSolver() exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "Error: No solutions") {
		t.Fatalf("runWithSolver() stdout = %q, want No solutions error", stdout.String())
	}
	if solver.calls != 2 {
		t.Errorf("solver calls = %d, want 2", solver.calls)
	}
}

func TestRunCtrlCBacktracksDuringStateInput(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
state "Done" as s1
[*] --> s0
s0 --> s1 : go
@enduml
`)
	stdout := &lockedBuffer{}
	interrupts := make(chan os.Signal, 1)
	inputReader, inputWriter := ioPipe(t)
	done := make(chan int, 1)
	go func() {
		done <- Run([]string{path}, inputReader, stdout, &bytes.Buffer{}, interrupts)
	}()

	writeAndWait(t, inputWriter, "[]\n", stdout, "command> ")
	writeAndWait(t, inputWriter, "s 0\n", stdout, "state> ")
	interrupts <- os.Interrupt
	waitFor(t, stdout, "State: Initial (s0)")
	_ = inputWriter.Close()

	if exitCode := <-done; exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
}

func TestRunInitialCtrlCIsFatal(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
[*] --> s0
@enduml
`)
	stdout := &bytes.Buffer{}
	interrupts := make(chan os.Signal, 1)
	interrupts <- os.Interrupt

	exitCode := Run([]string{path}, strings.NewReader(""), stdout, &bytes.Buffer{}, interrupts)

	if exitCode != 1 {
		t.Fatalf("Run() exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stdout.String(), "Fatal: No solutions found") {
		t.Errorf("Run() stdout = %q, want fatal error", stdout.String())
	}
}

func TestRunDoesNotSubmitPartialLineAtEOF(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
s0: value
[*] --> s0
@enduml
`)
	stdout := &bytes.Buffer{}

	exitCode := Run([]string{path}, strings.NewReader("[1]"), stdout, &bytes.Buffer{}, nil)

	if exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if strings.Contains(stdout.String(), "value = 1") {
		t.Errorf("Run() submitted a line without Enter:\n%s", stdout.String())
	}
}

func TestRunRejectsBadInvocationAndBadFiles(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing argument"},
		{name: "too many arguments", args: []string{"a", "b"}},
		{name: "missing file", args: []string{filepath.Join(t.TempDir(), "missing.puml")}},
		{name: "invalid file", args: []string{writeDiagram(t, "not PlantUML")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			if exitCode := Run(tt.args, strings.NewReader(""), &bytes.Buffer{}, stderr, nil); exitCode != 1 {
				t.Errorf("Run() exit code = %d, want 1", exitCode)
			}
			if stderr.Len() == 0 {
				t.Error("Run() stderr is empty, want error")
			}
		})
	}
}

func TestLoadDiagramReadsPlantUMLPNG(t *testing.T) {
	diagram, err := loadDiagram(filepath.Join("..", "..", "examples", "valid", "client.png"))
	if err != nil {
		t.Fatalf("loadDiagram() error = %v", err)
	}
	if len(diagram.States) == 0 {
		t.Fatal("loadDiagram() returned a diagram without states")
	}
}

func TestJSONPostSolver(t *testing.T) {
	group := core.State{
		ID:   "s0",
		Name: "Initial",
		Vars: []core.StateVar{{Name: "a"}, {Name: "b"}},
	}
	result := (JSONPostSolver{}).Solve(PostSolverInput{
		StateGroup:    group,
		EncodedValues: `[1, {"nested": ["ok"]}]`,
	})

	if result.Kind != PostSolverResultOK {
		t.Fatalf("Solve() kind = %v, want OK; err = %v", result.Kind, result.Err)
	}
	if len(result.State.Values) != 2 || result.State.Values[0].Name != "a" || result.State.Values[1].Name != "b" {
		t.Errorf("Solve() state values = %#v", result.State.Values)
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input     string
		wantKind  commandKind
		wantIndex int
		wantError string
	}{
		{input: "", wantKind: commandEmpty},
		{input: "l", wantKind: commandList},
		{input: "help", wantKind: commandHelp},
		{input: "s 12", wantKind: commandSelect, wantIndex: 12},
		{input: "j 0", wantKind: commandJump},
		{input: "s", wantError: "required an index of transition"},
		{input: "j -1", wantError: "Not a natural number"},
		{input: "t extra", wantError: "invalid arguments"},
		{input: "wat", wantError: "invalid command"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			kind, index, err := parseCommand(tt.input)
			if tt.wantError != "" {
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("parseCommand() error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCommand() error = %v", err)
			}
			if kind != tt.wantKind || index != tt.wantIndex {
				t.Errorf("parseCommand() = (%v, %d), want (%v, %d)", kind, index, tt.wantKind, tt.wantIndex)
			}
		})
	}
}

type sequenceSolver struct {
	results []PostSolverResult
	calls   int
}

func (s *sequenceSolver) Solve(PostSolverInput) PostSolverResult {
	result := s.results[s.calls]
	s.calls++
	return result
}

func writeDiagram(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "diagram.puml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ioPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	return reader, writer
}

func writeAndWait(t *testing.T, writer *os.File, input string, stdout *lockedBuffer, want string) {
	t.Helper()
	if _, err := writer.WriteString(input); err != nil {
		t.Fatal(err)
	}
	waitFor(t, stdout, want)
}

func waitFor(t *testing.T, stdout *lockedBuffer, want string) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		if strings.Contains(stdout.String(), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("stdout did not contain %q: %s", want, stdout.String())
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
