package csdfslicecmd

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

	chain := filepath.Join("testdata", "chain.puml")

	testCases := map[string]testCase{
		"context 0: the given state alone, every edge cut (lower boundary value)": {
			Args: []string{"-states", "s1", "-context", "0", chain},
			Want: `@startuml
state "Middle" as s1
s1 : n ; N
' CSDF-IGNORE-BEGIN
state "Start …" as s0_cut
state "End …" as s2_cut
[*] --> s0_cut
s0_cut --> s1 : a ; ; $n' = 0
s1 --> s2_cut : b ; $n = 0
' CSDF-IGNORE-END
@enduml
`,
		},
		"the initial state inside: the start edge kept, the channels note pruned (representative value)": {
			Args: []string{"-states", "s0,s1", "-context", "0", chain},
			Want: `@startuml
state "Start" as s0
state "Middle" as s1
s1 : n ; N
[*] --> s0
s0 --> s1 : a ; ; $n' = 0
note as channels
  channel a
end note
' CSDF-IGNORE-BEGIN
state "End …" as s2_cut
s1 --> s2_cut : b ; $n = 0
s2_cut --> s0 : c
' CSDF-IGNORE-END
@enduml
`,
		},
		"context 1 reaches both ways round the cycle: nothing cut (representative value)": {
			Args: []string{"-states", "s1", chain},
			Want: `@startuml
state "Start" as s0
state "Middle" as s1
s1 : n ; N
state "End" as s2
[*] --> s0
s0 --> s1 : a ; ; $n' = 0
s1 --> s2 : b ; $n = 0
s2 --> s0 : c
note as channels
  channel a
  channel b
  channel c
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
		"an unknown state": {
			Args:       []string{"-states", "s1,nowhere", filepath.Join("testdata", "chain.puml")},
			WantStderr: `unknown state "nowhere"`,
		},
		"text that is not a diagram": {
			Args:       []string{"-states", "s0"},
			Stdin:      "not a diagram\n",
			WantStderr: "expected @startuml at line 1, col 1",
		},
		"a global diagram": {
			Args:       []string{"-states", "s0", "../../../examples/promote/RECEIPTS.puml"},
			WantStderr: "run csdfpromote on it first",
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
			if !strings.Contains(spy.Stderr.String(), testCase.WantStderr) || strings.Contains(spy.Stderr.String(), "core.") {
				t.Errorf("want %q on stderr, got %q", testCase.WantStderr, spy.Stderr.String())
			}
			if spy.Stdout.String() != "" {
				t.Errorf("want nothing on stdout, got %q", spy.Stdout.String())
			}
		})
	}
}
