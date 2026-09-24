package csdftranstablecmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncOK(t *testing.T) {
	type testCase struct {
		Args       []string
		Stdin      string
		WantStdout string
		WantStderr string
	}

	testCases := map[string]testCase{
		"a file argument, with the defaults": {
			Args: []string{filepath.Join("testdata", "a.puml")},
			WantStdout: "state\tname\tinsert(coin)\treset\n" +
				"a\ta\t\"[\"\"coins' is {coin}\"\"(c, x, x')] → b\"\t×\n" +
				"b\tb\t×\t→ a\n",
		},
		"-expr-mode logical": {
			Stdin: `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : insert(coin) ; g
@enduml
`,
			Args: []string{"-expr-mode", "logical"},
			WantStdout: "state\tname\tinsert(coin)\n" +
				"a\ta\t\"[\"\"g\"\"(c, x)] → b\n[¬\"\"g\"\"(c, x)] ×\"\n" +
				"b\tb\t×\n",
		},
		"the ID of an unreachable state is written to standard error": {
			Stdin: `@startuml
state "S0" as s0
state "Z" as z
[*] --> s0
@enduml
`,
			WantStdout: "state\tname\n" +
				"s0\tS0\n",
			WantStderr: "warning: unreachable states have no row: z\n",
		},
		"-v (representative value)": {
			Args:       []string{"-v"},
			WantStdout: version.Version + "\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Act
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Errorf("want 0, got %d", exitStatus)
			}
			if diff := cmp.Diff(testCase.WantStdout, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
			if diff := cmp.Diff(testCase.WantStderr, spy.Stderr.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncNG(t *testing.T) {
	type testCase struct {
		Stdin      string
		Args       []string
		WantStderr string
	}

	testCases := map[string]testCase{
		"a reachable tau cycle": {
			Stdin: `@startuml
state "A" as A
state "B" as B
[*] --> A
A --> B : tau
B --> A : tau
@enduml
`,
			WantStderr: "the tau cycle A -> B -> A is reachable",
		},
		"a parse error": {
			Stdin:      "not a diagram\n",
			WantStderr: "Error: ",
		},
		// A global diagram's edges are not the whole of its behaviour, so every
		// tool but csdfpromote has to refuse one.
		"a global diagram": {
			Args:       []string{"../../../examples/promote/TRADES.puml"},
			WantStderr: "run csdfpromote on it first",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Act
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
