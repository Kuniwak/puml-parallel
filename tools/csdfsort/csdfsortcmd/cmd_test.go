package csdfsortcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncOK(t *testing.T) {
	type testCase struct {
		Args  []string
		Stdin string
		Want  string
	}

	testCases := map[string]testCase{
		"a file argument (a hand-written diagram in authoring order)": {
			Args: []string{filepath.Join("testdata", "unsorted.puml")},
			Want: `@startuml
state "First" as s0
state "Second" as s1
[*] --> s1
s0 --> s1 : a
s1 --> s0 : a ; g
s1 --> s0 : b
@enduml
`,
		},
		"standard input (equivalent to a file argument)": {
			Args: []string{},
			Stdin: `@startuml
state "s1" as s1
state "s0" as s0
[*] --> s0
s0 --> s1 : a
@enduml
`,
			Want: `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`,
		},
		"a diagram whose clauses use the builtins (clauses kept as written)": {
			Args: []string{filepath.Join("testdata", "builtins.puml")},
			Want: `@startuml
state "Door" as door
door : doors ; doors
door : loc ; loc
state "Shop" as shop
shop : doors ; doors
shop : loc ; loc
[*] --> shop : @empty(doors') ∧ $loc' = 0
door --> shop : close ; @ne(loc, 0) ; @eq(doors', doors) ∧ @eq(loc', loc)
shop --> shop : add?k ; @notin(k, doors) ; @update(doors', doors, k, k) ∧ @eq(loc', loc)
shop --> door : open?k ; @in(k, doors) ∧ @at(doors[k], shut) ; @frame(doors', doors, k) ∧ @at(doors'[k], wide) ∧ @eq(loc', k)
note as channels
  channel add : k
  channel open : k
end note
@enduml
`,
		},
		"-v (representative value)": {
			Args: []string{"-v"},
			Want: version.Version + "\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Errorf("want 0, got %d", exitStatus)
			}
			if diff := cmp.Diff(testCase.Want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncRefuses(t *testing.T) {
	type testCase struct {
		Args       []string
		Stdin      string
		WantStderr string
	}

	testCases := map[string]testCase{
		"@opaque": {
			Args:       []string{filepath.Join("testdata", "old_opaque.puml")},
			WantStderr: "unknown tool word @opaque",
		},
		"a renaming @[x := 1]": {
			Args:       []string{filepath.Join("testdata", "old_update.puml")},
			WantStderr: "unknown tool word @ at 0",
		},
		"text that is not a diagram": {
			Args:       []string{},
			Stdin:      "not a diagram\n",
			WantStderr: "expected @startuml at line 1, col 1",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 1 {
				t.Errorf("want 1, got %d", exitStatus)
			}
			if !strings.Contains(spy.Stderr.String(), testCase.WantStderr) {
				t.Errorf("want %q on stderr, got %q", testCase.WantStderr, spy.Stderr.String())
			}
			if spy.Stdout.String() != "" {
				t.Errorf("want nothing on stdout, got %q", spy.Stdout.String())
			}
		})
	}
}

// A global diagram's edges are not the whole of its behaviour, so every tool but
// csdfpromote has to refuse one. The refusal comes from the reader, which every
// tool goes through, and it says what the author is missing.
func TestNewMainFuncRefusesAGlobalDiagram(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Execute
	exitStatus := cmdFunc([]string{"../../../examples/promote/TRADES.puml"}, spy.New())

	// Assert
	if exitStatus != 1 {
		t.Fatalf("want 1, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stderr.String(), "run csdfpromote on it first") {
		t.Errorf("want the hint on stderr, got %q", spy.Stderr.String())
	}
}
