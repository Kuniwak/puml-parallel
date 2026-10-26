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
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func mustClause(t *testing.T, text string) clause.And {
	t.Helper()
	c, err := clause.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEventDisplaysGolden(t *testing.T) {
	state := animation.State{
		ID:   "review",
		Name: "Review order",
		Values: []animation.Value{
			{Name: "count", Value: 2},
			{Name: "metadata", Value: map[string]any{"priority": true, "tags": []any{"new", "gift"}}},
		},
	}
	history := []animation.HistoryEntry{
		{
			State: animation.State{
				ID:     "draft",
				Name:   "Draft order",
				Values: []animation.Value{{Name: "count", Value: 1}},
			},
			Trace: []string{},
		},
		{
			State: state,
			Trace: []string{"submit?order", "approve"},
		},
	}

	tests := []struct {
		name    string
		diagram *core.Diagram
		prompt  string
		display func(*repl)
	}{
		{
			name:   "EventDisplayStateGroup",
			prompt: "state> ",
			display: func(r *repl) {
				r.displayStateValuePrompt(animation.Pending{
					Target: animation.Target{ID: "approved", Name: "Approved", Vars: []string{"status"}},
					Guard:  mustClause(t, "$count > 0 ∧ @ne(count, 9)"),
					Post:   mustClause(t, `$status' = "reviewing"`),
					Prev:   &state,
				})
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
			diagram: &core.Diagram{
				States: []string{"review", "approved", "rejected"},
				Labels: map[string]string{"review": "Review order", "approved": "Approved", "rejected": "Rejected"},
				Edges: []core.Edge{
					{
						Src:   "review",
						Dst:   "approved",
						Event: core.Event{Channel: "approve"},
						Guard: mustClause(t, "$count > 0"),
						Post:  mustClause(t, `$status' = "approved"`),
					},
					{
						Src:   "review",
						Dst:   "rejected",
						Event: core.Event{Channel: "reject", Fields: []core.Field{{Name: "reason"}}},
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
			prompt:  "command> ",
			display: func(r *repl) {
				r.displayState(state)
			},
		},
		{
			name:   "EventDisplayTrace",
			prompt: "command> ",
			display: func(r *repl) {
				r.displayTrace([]string{"submit?order", "approve"})
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
				diagram = &core.Diagram{}
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
s0: count
state "Done" as s1
s1: result
[*] --> s0 : $count' = 0
s0 --> s1 : insert?coin ; @ne(count, 9) ∧ $count >= 0 ; $result' is done
@enduml
`)
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("[0]\nl\nt\nh\ns 0\n[\"ok\"]\nt\nj 0\n\n"))

	err := runREPL(path, spy.New(), nil, runOptions{Suggesters: animation.NoSuggesters})

	if err != nil {
		t.Fatalf("runREPL() error = %v; stderr = %q", err, spy.Stderr.String())
	}
	if spy.Stderr.Len() != 0 {
		t.Errorf("runREPL() stderr = %q, want empty", spy.Stderr.String())
	}
	for _, want := range []string{
		"State: Initial",
		"    count'\n",
		"[0] insert?coin -> Done",
		"Guard: @ne(count, 9) ∧ $count >= 0",
		`"insert?coin"`,
		"State: Done",
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

	err := runREPL(path, spy.New(), nil, runOptions{Suggesters: animation.NoSuggesters})

	if err != nil {
		t.Fatalf("runREPL() error = %v; stderr = %q", err, spy.Stderr.String())
	}
	for _, want := range []string{
		"invalid JSON array",
		"expected 1 value(s) for [value], got 0",
		"null is not a valid state variable value",
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
		done <- runREPL(path, spy.New(), interrupts, runOptions{Suggesters: animation.NoSuggesters})
	}()

	writeAndWait(t, inputWriter, "[]\n", spy.Stdout, "command> ")
	writeAndWait(t, inputWriter, "s 0\n", spy.Stdout, "state> ")
	interrupts <- os.Interrupt
	waitFor(t, spy.Stdout, "State: Initial")
	_ = inputWriter.Close()

	if err := <-done; err != nil {
		t.Fatalf("runREPL() error = %v, want nil", err)
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

	err := runREPL(path, spy.New(), interrupts, runOptions{Suggesters: animation.NoSuggesters})

	if err == nil {
		t.Fatalf("runREPL() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "No solutions found") {
		t.Errorf("runREPL() error = %v, want fatal error mentioning No solutions found", err)
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

	err := runREPL(path, spy.New(), nil, runOptions{Suggesters: animation.NoSuggesters})

	if err != nil {
		t.Fatalf("runREPL() error = %v, want nil", err)
	}
	if strings.Contains(stdout.String(), "value = 1") {
		t.Errorf("runREPL() submitted a line without Enter:\n%s", stdout.String())
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

func TestRunREPLRejectsBadFiles(t *testing.T) {
	tests := map[string]struct {
		file string
		want string
	}{
		"missing file": {file: filepath.Join(t.TempDir(), "missing.puml"), want: "cannot read the file"},
		"invalid file": {file: writeDiagram(t, "not PlantUML"), want: "cannot read the diagram"},
		"a clause of the old syntax": {
			file: writeDiagram(t, "@startuml\nstate \"S\" as s\n[*] --> s\ns --> s : a ; @opaque\n@enduml\n"),
			want: "cannot read the diagram: animation.ReadDiagram: coreio.ReadBytes: coreio.Read:",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			spy := cli.SpyProcInout()

			// Execute
			err := runREPL(tt.file, spy.New(), nil, runOptions{Suggesters: animation.NoSuggesters})

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("runREPL() error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestRunStepsADiagramOfTheNewBuiltins(t *testing.T) {
	// Setup
	path := writeDiagram(t, `@startuml
state "Open" as s0
s0: m
s0: k
[*] --> s0 : @empty(m') ∧ @eq(k', 0)
s0 --> s0 : put?x ; @notin(m, x) ∧ @ne(x, k) ; @update(m', m, x, 1) ∧ @eq(k', x)
@enduml
`)
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("[{}, 0]\ns 0\n[{\"1\": 1}, 1]\nt\n"))

	// Execute
	err := runREPL(path, spy.New(), nil, runOptions{Suggesters: animation.NoSuggesters})

	// Assert
	if err != nil {
		t.Fatalf("runREPL() error = %v", err)
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"Post Condition:\n  @empty(m') ∧ @eq(k', 0)\n",
		"[0] put?x -> Open\n      Guard: @notin(m, x) ∧ @ne(x, k)\n      Post: @update(m', m, x, 1) ∧ @eq(k', x)\n",
		"  m = {\"1\":1}\n  k = 1\n",
		`"put?x"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, out)
		}
	}
}

func TestRunExploresTheDiagramJSONOfAnExpansion(t *testing.T) {
	// Setup
	raw, err := core.MarshalDiagram(&core.Diagram{
		States: []string{"G"}, Vars: map[string][]string{"G": {"accounts"}}, Init: "G", InitPost: clause.And{},
		Families: []core.FamilyStates{{Map: "accounts", Init: "none", States: []string{"none", "opened"},
			Vars: map[string][]string{"opened": {"balance"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(t.TempDir(), "expansion.json")
	if err := os.WriteFile(jsonPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeDiagram(t, "@startuml\nstate \"S\" as s\n[*] --> s\n@enduml\n")
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(`[{"a": {"at": "opened", "fields": {}}}]` + "\n" + `[{"a": {"at": "opened", "fields": {"balance": 1}}}]` + "\n"))

	// Execute
	err = runREPL(path, spy.New(), nil, runOptions{DiagramJSON: jsonPath, Suggesters: animation.NoSuggesters})

	// Assert
	if err != nil {
		t.Fatalf("runREPL() error = %v", err)
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"    accounts', a map from IDs to instances {\"at\": <state>, \"fields\": {...}}:\n      at opened, holding balance\n",
		`Error: accounts["a"]: an instance at opened holds the fields [balance], got []`,
		`State: G`,
		`accounts = {"a":{"at":"opened","fields":{"balance":1}}}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, out)
		}
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

func TestRunReplaysAFindingBeforeTheFirstPrompt(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "counting" as a
a: n
state "stopped" as b
[*] --> a : $n' = 2
a --> a : dec ; $n > 0 ; $n' = $n - 1
a --> b : stop ; $n = 0
@enduml
`)
	replay := animation.Replay{Initial: []byte(`[2]`), Steps: []animation.ReplayStep{
		{Event: "dec", Src: "a", Dst: "a", Line: 6, Values: []byte(`[1]`)},
		{Event: "dec", Src: "a", Dst: "a", Line: 6, Values: []byte(`[0]`)},
		{Event: "stop", Src: "a", Dst: "b", Line: 7, Values: []byte(`[]`)},
	}}
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("t\nh\n"))

	err := runREPL(path, spy.New(), nil, runOptions{Replay: &replay, Suggesters: animation.NoSuggesters})

	if err != nil {
		t.Fatalf("runREPL() error = %v; stderr = %q", err, spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{"Replayed 3 step(s)", "State: stopped", `"dec"`, `"stop"`, "History:"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "state> ") && strings.Index(out, "state> ") < strings.Index(out, "State: stopped") {
		t.Errorf("a value prompt came before the replayed state:\n%s", out)
	}
}

type stubSuggester struct {
	initial []animation.Suggestion
	next    []animation.Suggestion
}

func (s stubSuggester) Initial() ([]animation.Suggestion, error) { return s.initial, nil }
func (s stubSuggester) Next(animation.State, int) ([]animation.Suggestion, error) {
	return s.next, nil
}

func TestRunShowsSuggestionsAndTakesOneWithHash(t *testing.T) {
	path := writeDiagram(t, `@startuml
state "counting" as a
a: n
state "stopped" as b
[*] --> a : $n' = 2
a --> a : dec ; $n > 0 ; $n' = $n - 1
a --> b : stop ; $n = 0
@enduml
`)
	sg := stubSuggester{
		initial: []animation.Suggestion{{Values: []byte(`[2]`)}},
		next:    []animation.Suggestion{{Values: []byte(`[1]`)}},
	}
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("#0\ns 0\n#9\n#0\nl\n"))

	err := runREPL(path, spy.New(), nil, runOptions{Suggesters: func([]byte, string) (animation.Suggester, func(), error) { return sg, animation.NoCleanup, nil }})

	if err != nil {
		t.Fatalf("runREPL() error = %v; stderr = %q", err, spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{"Suggested by csdfbmc", "[#0] [2]", "n = 2", "[#0] [1]", "no suggestion #9", "n = 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, out)
		}
	}
}
