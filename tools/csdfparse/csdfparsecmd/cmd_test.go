package csdfparsecmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncPrintsTheDiagramAsJSON(t *testing.T) {
	tests := map[string]struct {
		args  []string
		stdin string
		want  string
	}{
		"a diagram on standard input, its clauses as formulas": {
			stdin: `@startuml
state "Initial" as s0
s0: n
state "Done" as s1
[*] --> s0 : $n' == 0
s0 --> s1 : finish?result ; $n < $result
s1 --> [*]
@enduml
`,
			want: `{"version": 1, "name": "", "states": ["s0", "s1"], "labels": {"s0": "Initial", "s1": "Done"},
"vars": {"s0": ["n"], "s1": []}, "init": "s0", "init_line": 5,
"init_post": [{"kind": "atom", "pieces": ["", " == 0"], "holes": [0], "args": [{"kind": "free", "name": "n", "primed": true}],
  "text": ["", " == 0"], "primed": [true], "function": "$_0 == 0"}],
"edges": [{"src": "s0", "dst": "s1", "channel": "finish", "fields": [{"dir": "?", "name": "result"}], "label": "finish?result",
  "guard": [{"kind": "atom", "pieces": ["", " < ", ""], "holes": [0, 1], "args": [{"kind": "free", "name": "n"}, {"kind": "free", "name": "result"}],
    "text": ["", " < ", ""], "primed": [false, false], "function": "$_0 < $_1"}],
  "post": [], "line": 6}],
"ends": [{"src": "s1", "guard": []}], "assumes": []}`,
		},
		"a file argument, as standard input": {
			args: []string{"../../../examples/valid/skip.puml"},
			want: `{"version": 1, "name": "", "states": ["s0"], "labels": {"s0": "SKIP"}, "vars": {"s0": []}, "init": "s0", "init_post": [], "init_line": 5,
"edges": [], "ends": [{"src": "s0", "guard": []}], "assumes": []}`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(tt.stdin))

			// Execute
			exitStatus := cmdFunc(tt.args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Fatalf("want 0, got %d: %s", exitStatus, spy.Stderr.String())
			}
			var got, want any
			if err := json.Unmarshal([]byte(spy.Stdout.String()), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Error(diff)
			}
			if spy.Stderr.Len() != 0 {
				t.Errorf("want empty stderr, got %q", spy.Stderr.String())
			}
		})
	}
}

func TestNewMainFuncRefusesADiagramThatStillHoldsDirectives(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Execute
	exitStatus := cmdFunc([]string{"../../../examples/promote/TRADES.puml"}, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Fatal("want a nonzero exit status")
	}
	if !strings.Contains(spy.Stderr.String(), "csdfpromote") {
		t.Errorf("want a hint to run csdfpromote first, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"-v"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	want := version.Version + "\n"
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}
