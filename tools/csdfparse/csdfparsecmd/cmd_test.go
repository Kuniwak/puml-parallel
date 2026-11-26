package csdfparsecmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncPrintsTheSyntaxTreeAsJSON(t *testing.T) {
	tests := map[string]struct {
		args  []string
		stdin string
		want  string
	}{
		"a plain diagram on standard input, its clauses as written": {
			stdin: `@startuml D
' a comment
state "Initial" as s0
s0 : n ; N
[*] --> s0 : $n' == 0
s0 --> s0 : finish?r ; $n < $r
s0 --> [*]
note as channels
  channel finish : N
end note
@enduml
`,
			want: `{"kind": "csdf-syntax-tree", "version": 1, "name": "D", "line": 1, "col": 1, "body": [
  {"kind": "comment", "line": 2, "col": 1, "text": " a comment", "block": false},
  {"kind": "state", "line": 3, "col": 1, "id": "s0", "label": "Initial"},
  {"kind": "state_var", "line": 4, "col": 1, "state": "s0", "name": "n", "type": "N"},
  {"kind": "start_edge", "line": 5, "col": 1, "dst": "s0", "post": {"line": 5, "col": 14, "text": "$n' == 0", "conjuncts": [
    {"kind": "atom", "offset": 0, "parts": [
      {"kind": "hole", "offset": 0, "braced": false, "term": {"kind": "var", "offset": 1, "name": "n", "primed": true}},
      {"kind": "text", "text": " == 0"}]}]}},
  {"kind": "edge", "line": 6, "col": 1, "src": "s0", "dst": "s0",
    "event": {"line": 6, "col": 13, "text": "finish?r", "channel": "finish", "fields": [{"dir": "?", "name": "r"}]},
    "guard": {"line": 6, "col": 24, "text": "$n < $r", "conjuncts": [
      {"kind": "atom", "offset": 0, "parts": [
        {"kind": "hole", "offset": 0, "braced": false, "term": {"kind": "var", "offset": 1, "name": "n", "primed": false}},
        {"kind": "text", "text": " < "},
        {"kind": "hole", "offset": 5, "braced": false, "term": {"kind": "var", "offset": 6, "name": "r", "primed": false}}]}]},
    "post": null},
  {"kind": "end_edge", "line": 7, "col": 1, "src": "s0", "guard": null},
  {"kind": "note", "line": 8, "col": 1, "id": "channels", "lines": [
    {"kind": "channel", "line": 9, "col": 3, "name": "finish", "fields": ["N"]}]}]}`,
		},
		"a global diagram, its directives and pragmas as written, and the path of its file": {
			args: []string{"testdata/global.puml"},
			want: `{"kind": "csdf-syntax-tree", "version": 1, "path": "testdata/global.puml", "name": "G", "line": 1, "col": 1, "body": [
  {"kind": "pragma", "line": 2, "col": 1, "name": "define", "arg": "PROMOTED"},
  {"kind": "state", "line": 3, "col": 1, "id": "running", "label": "稼働中", "body": [
    {"kind": "state_var", "line": 4, "col": 3, "state": "running", "name": "users", "type": "UserId ⇸ User"},
    {"kind": "state", "line": 5, "col": 3, "id": "runningUsers", "label": "users : (利用者ID : UserId) ⇸ User", "stereotype": "promote", "body": [
      {"kind": "pragma", "line": 6, "col": 5, "name": "include", "arg": "local/USER.puml"}]}]},
  {"kind": "start_edge", "line": 9, "col": 1, "dst": "running", "post": null},
  {"kind": "note", "line": 10, "col": 1, "side": "bottom", "of": "runningUsers", "lines": [
    {"kind": "constrain", "line": 11, "col": 3,
      "event": {"line": 11, "col": 3, "text": "JOIN?利用者ID", "channel": "JOIN", "fields": [{"dir": "?", "name": "利用者ID"}]},
      "guard": {"line": 11, "col": 3, "text": "ok", "conjuncts": [{"kind": "atom", "offset": 0, "parts": [{"kind": "text", "text": "ok"}]}]}},
    {"kind": "text", "line": 12, "col": 3, "text": "why it is so"}]},
  {"kind": "note", "line": 14, "col": 1, "id": "s1", "lines": [
    {"kind": "sync", "line": 15, "col": 3, "event": "LOG", "targets": [{"map": "users", "id": "利用者ID"}], "guard": null}]}]}`,
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

func TestNewMainFuncParsesEveryExample(t *testing.T) {
	for _, path := range []string{
		"../../../examples/promote/RECEIPTS.puml",
		"../../../examples/promote/MODES.puml",
		"../../../examples/promote/local/CYCLE.puml",
		"../../../examples/valid/skip.puml",
	} {
		t.Run(path, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()

			// Execute
			exitStatus := cmdFunc([]string{path}, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Fatalf("want 0, got %d: %s", exitStatus, spy.Stderr.String())
			}
		})
	}
}

func TestNewMainFuncRefusesASyntaxError(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("@startuml\nbogus\n@enduml\n"))

	// Execute
	exitStatus := cmdFunc(nil, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Fatal("want a nonzero exit status")
	}
	if !strings.Contains(spy.Stderr.String(), "line 2") {
		t.Errorf("want the line of the error, got %q", spy.Stderr.String())
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
