package csdfreplcmd

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestEventDisplaysGolden(t *testing.T) {
	state := csdf.RuntimeState{
		ID:   "review",
		Name: "Review order",
		Values: []csdf.StateValue{
			{Name: "count", Value: 2},
			{Name: "metadata", Value: map[string]any{"priority": true, "tags": []any{"new", "gift"}}},
		},
	}
	history := []HistoryEntry{
		{
			State: csdf.RuntimeState{
				ID:     "draft",
				Name:   "Draft order",
				Values: []csdf.StateValue{{Name: "count", Value: 1}},
			},
			Trace: []csdf.Event{},
		},
		{
			State: state,
			Trace: []csdf.Event{"submit(order)", "approve"},
		},
	}

	tests := []struct {
		name    string
		diagram *csdf.Diagram
		prompt  string
		display func(*repl)
	}{
		{
			name:   "EventDisplayStateGroup",
			prompt: "state> ",
			display: func(r *repl) {
				r.displayStateValuePrompt(&state, csdf.State{
					ID:   "approved",
					Name: "Approved",
					Vars: []csdf.StateVar{
						{Name: "status", Type: "string"},
					},
				}, "count > 0", `status' = "reviewing"`)
			},
		},
		{
			name:   "EventDisplayStateVarsError",
			prompt: "state> ",
			display: func(r *repl) {
				r.displayError("expected 1 value(s) for [count], got 2")
			},
		},
		{
			name:   "EventDisplayCommandError",
			prompt: "command> ",
			display: func(r *repl) {
				r.displayError("invalid command")
			},
		},
		{
			name:   "EventDisplayTrans",
			prompt: "command> ",
			diagram: &csdf.Diagram{
				States: map[csdf.StateID]csdf.State{
					"approved": {ID: "approved", Name: "Approved"},
					"rejected": {ID: "rejected", Name: "Rejected"},
				},
				Edges: []csdf.Edge{
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
			diagram: &csdf.Diagram{},
			prompt:  "command> ",
			display: func(r *repl) {
				r.displayState(state)
			},
		},
		{
			name:   "EventDisplayTrace",
			prompt: "command> ",
			display: func(r *repl) {
				r.displayTrace([]csdf.Event{"submit(order)", "approve"})
			},
		},
		{
			name:   "EventDisplayHistory",
			prompt: "command> ",
			display: func(r *repl) {
				r.displayHistory(history)
			},
		},
		{
			name:   "EventDisplayHelp",
			prompt: "command> ",
			display: func(r *repl) {
				r.displayHelp()
			},
		},
		{
			name:   "EventDisplayEmptyLine",
			prompt: "command> ",
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
				diagram = &csdf.Diagram{}
			}
			lines := make(chan lineResult, 1)
			lines <- lineResult{}
			close(lines)
			r := &repl{diagram: diagram, stdout: stdout, lines: lines}
			tt.display(r)
			if tt.prompt != "" {
				if _, outcome, _ := r.readLine(tt.prompt); outcome != inputLine {
					t.Fatalf("readLine() outcome = %v, want inputLine", outcome)
				}
			}
			assertGolden(t, tt.name, stdout.Bytes())
		})
	}
}

func TestDisplayStateValuePromptForInitialState(t *testing.T) {
	stdout := &bytes.Buffer{}
	lines := make(chan lineResult, 1)
	lines <- lineResult{}
	close(lines)
	r := &repl{stdout: stdout, lines: lines}

	r.displayStateValuePrompt(nil, csdf.State{
		ID:   "initial",
		Name: "Initial",
		Vars: []csdf.StateVar{
			{Name: "count", Type: "number"},
			{Name: "metadata"},
		},
	}, "", "")
	if _, outcome, _ := r.readLine("state> "); outcome != inputLine {
		t.Fatalf("readLine() outcome = %v, want inputLine", outcome)
	}

	want := "" +
		"State: (none)\n" +
		"\n" +
		"Guard:\n" +
		"  true\n" +
		"\n" +
		"Post State Group:\n" +
		"  Initial\n" +
		"    count' as number\n" +
		"    metadata' as any\n" +
		"\n" +
		"Post Condition:\n" +
		"  true\n" +
		"\n" +
		"Enter 2 values as a JSON array in declaration order: [<count>, <metadata>].\n" +
		"\n" +
		"state> \n"
	if stdout.String() != want {
		t.Error(cmp.Diff(want, stdout.String()))
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
		t.Log("update with: go test ./tools/csdfrepl -update")
		t.Error(cmp.Diff(expected, actual))
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
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("[0]\nl\nt\nh\ns 0\n[\"ok\"]\nt\nj 0\n\n"))

	err := runWithSolver(path, spy.New(), nil, csdf.SolveJSON)

	if err != nil {
		t.Fatalf("runWithSolver() error = %v; stderr = %q", err, spy.Stderr.String())
	}
	if spy.Stderr.Len() != 0 {
		t.Errorf("runWithSolver() stderr = %q, want empty", spy.Stderr.String())
	}
	for _, want := range []string{
		"State: Initial (s0)",
		"count' as number",
		"[0] insert(coin) -> Done (s1)",
		`"insert(coin)"`,
		"State: Done (s1)",
		`result = "ok"`,
		"Deadlock: no outgoing transitions.",
		"History:",
	} {
		if !strings.Contains(spy.Stdout.String(), want) {
			t.Errorf("Run() stdout does not contain %q:\n%s", want, spy.Stdout.String())
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
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("null\n{}\n[]\n[null]\n[1]\ns\ns nope\ns 0\nj\nj nope\nj 9\nl extra\nunknown\n"))

	err := runWithSolver(path, spy.New(), nil, csdf.SolveJSON)

	if err != nil {
		t.Fatalf("runWithSolver() error = %v; stderr = %q", err, spy.Stderr.String())
	}
	for _, want := range []string{
		"invalid JSON array",
		"expected 1 value(s) for [value], got 0",
		"null is not a supported JSON value",
		"required an index of transition",
		"Not a natural number",
		"Index out of range",
		"required an index of history",
		"invalid arguments",
		"invalid command",
	} {
		if !strings.Contains(spy.Stdout.String(), want) {
			t.Errorf("Run() stdout does not contain %q:\n%s", want, spy.Stdout.String())
		}
	}
}

func TestRunKeepsNoSolutionsEscapeHatch(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
[*] --> s0
@enduml
`)
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("[]\n[]\n"))
	solver, calls := sequenceSolver(
		csdf.PostSolverResult{Kind: csdf.PostSolverResultNoSolutions},
		csdf.PostSolverResult{
			Kind:  csdf.PostSolverResultOK,
			State: csdf.RuntimeState{ID: "s0", Name: "Initial"},
		},
	)

	err := runWithSolver(path, spy.New(), nil, solver)

	if err != nil {
		t.Fatalf("runWithSolver() error = %v, want nil", err)
	}
	if !strings.Contains(spy.Stdout.String(), "Error: No solutions") {
		t.Fatalf("runWithSolver() stdout = %q, want No solutions error", spy.Stdout.String())
	}
	if *calls != 2 {
		t.Errorf("solver calls = %d, want 2", *calls)
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
	interrupts := make(chan os.Signal, 1)
	inputReader, inputWriter := ioPipe(t)
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(inputReader)
	done := make(chan error, 1)
	go func() {
		done <- runWithSolver(path, spy.New(), interrupts, csdf.SolveJSON)
	}()

	writeAndWait(t, inputWriter, "[]\n", spy.Stdout, "command> ")
	writeAndWait(t, inputWriter, "s 0\n", spy.Stdout, "state> ")
	interrupts <- os.Interrupt
	waitFor(t, spy.Stdout, "State: Initial (s0)")
	_ = inputWriter.Close()

	if err := <-done; err != nil {
		t.Fatalf("runWithSolver() error = %v, want nil", err)
	}
}

func TestRunInitialCtrlCIsFatal(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
[*] --> s0
@enduml
`)
	interrupts := make(chan os.Signal, 1)
	interrupts <- os.Interrupt
	spy := cli.SpyProcInout()

	err := runWithSolver(path, spy.New(), interrupts, csdf.SolveJSON)

	if err == nil {
		t.Fatalf("runWithSolver() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "No solutions found") {
		t.Errorf("runWithSolver() error = %v, want fatal error mentioning No solutions found", err)
	}
}

func TestRunDoesNotSubmitPartialLineAtEOF(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "Initial" as s0
s0: value
[*] --> s0
@enduml
`)
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("[1]"))
	stdout := &bytes.Buffer{}

	err := runWithSolver(path, spy.New(), nil, csdf.SolveJSON)

	if err != nil {
		t.Fatalf("runWithSolver() error = %v, want nil", err)
	}
	if strings.Contains(stdout.String(), "value = 1") {
		t.Errorf("runWithSolver() submitted a line without Enter:\n%s", stdout.String())
	}
}

func TestTerminalLineReaderEditsInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Ctrl+H",
			input: "ab\x08c\r",
			want:  "ac",
		},
		{
			name:  "Backspace",
			input: "ab\x7fc\r",
			want:  "ac",
		},
		{
			name:  "left arrow inserts before cursor",
			input: "ac\x1b[Db\r",
			want:  "abc",
		},
		{
			name:  "right arrow moves cursor",
			input: "ac\x1b[D\x1b[D\x1b[Cb\r",
			want:  "abc",
		},
		{
			name:  "cursor boundaries are ignored",
			input: "\x1b[D\x08a\x1b[C\r",
			want:  "a",
		},
		{
			name:  "Unicode runes",
			input: "あう\x1b[Dい\r",
			want:  "あいう",
		},
		{
			name:  "Ctrl+D deletes after cursor",
			input: "ac\x1b[D\x04\r",
			want:  "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(tt.input))
			reader := newTerminalLineReader(spy.New())
			got, err := reader.readLine("command> ")
			if err != nil {
				t.Fatalf("readLine() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("readLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTerminalLineReaderControlOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "Ctrl+C", input: "\x03", wantErr: errTerminalInterrupt},
		{name: "Ctrl+D on empty line", input: "\x04", wantErr: io.EOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(tt.input))
			reader := newTerminalLineReader(spy.New())
			_, err := reader.readLine("command> ")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("readLine() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunWithSolverRejectsBadFiles(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "missing file", file: filepath.Join(t.TempDir(), "missing.puml")},
		{name: "invalid file", file: writeDiagram(t, "not PlantUML")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := cli.SpyProcInout()
			err := runWithSolver(tt.file, spy.New(), nil, csdf.SolveJSON)
			if err == nil {
				t.Error("runWithSolver() error = nil, want error")
			}
		})
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

// sequenceSolver returns a csdf.PostSolver that yields the given results in
// order, plus a pointer to the number of times it has been called.
func sequenceSolver(results ...csdf.PostSolverResult) (csdf.PostSolver, *int) {
	calls := 0
	return func(csdf.PostSolverInput) csdf.PostSolverResult {
		result := results[calls]
		calls++
		return result
	}, &calls
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

func writeAndWait(t *testing.T, writer *os.File, input string, stdout *cli.LockedBuffer, want string) {
	t.Helper()
	if _, err := writer.WriteString(input); err != nil {
		t.Fatal(err)
	}
	waitFor(t, stdout, want)
}

func waitFor(t *testing.T, stdout *cli.LockedBuffer, want string) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		if strings.Contains(stdout.String(), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("stdout did not contain %q: %s", want, stdout.String())
}
