package csdfcompcmd

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// composedBody is everything after the @startuml line, whose diagram name
// records the command that generated the composition and so varies per case.
const composedBody = `state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
state "(s2, s1)" as s2_s1
state "(s2, s2)" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : tau
s2_s1 --> s2_s2 : out
@enduml
`

func TestNewMainFuncComposesATree(t *testing.T) {
	testCases := map[string]struct {
		Args  []string
		Stdin string
	}{
		"a tree file (paths resolved against the directory of the tree file)": {
			Args: []string{"../../../examples/valid/in_out_tree.json"},
		},
		"a tree on standard input (paths resolved against -base)": {
			Args: []string{"-base", "../../../examples/valid", "-"},
			Stdin: `{
				"op": "HIDE",
				"proc": {
					"op": "INTERFACE_PARALLEL",
					"sync": ["sync"],
					"procs": [
						{"op": "REFER", "path": "in.puml"},
						{"op": "REFER", "path": "out.puml"}
					]
				},
				"events": ["sync"]
			}`,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader(testCase.Stdin)

			// Act
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Errorf("want 0, got %d", exitStatus)
			}
			want := "@startuml " + tools.GeneratedBy("csdfcomp", testCase.Args) + "\n" + composedBody
			if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncReportsBrokenTrees(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = strings.NewReader(`{"op": "SEQ"}`)

	// Act
	exitStatus := cmdFunc([]string{"-"}, spy.New())

	// Assert
	if exitStatus != 1 {
		t.Errorf("want 1, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stderr.String(), "SEQ") {
		t.Errorf("want an error naming the unknown op, got %q", spy.Stderr.String())
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
