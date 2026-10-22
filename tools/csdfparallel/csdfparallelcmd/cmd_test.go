package csdfparallelcmd

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// recorded is the file at path, which a test compares untitled.
func recorded(t *testing.T, path string) string {
	bs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bs)
}

func TestNewMainFuncCompose(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "two diagrams synchronised on a channel",
			args: []string{"-sync", "sync", "../../../examples/valid/in.puml", "../../../examples/valid/out.puml"},
			want: `@startuml
state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
state "(s2, s1)" as s2_s1
state "(s2, s2)" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : sync
s2_s1 --> s2_s2 : out
@enduml
`,
		},
		{
			// A diagram whose clauses hold ∃, as csdfhide prints them.
			name: "what csdfhide printed",
			args: []string{"testdata/hidden.puml", "testdata/other.puml"},
			want: `@startuml
state "(a, b)" as a_b
a_b : x
[*] --> a_b : $x' = 0
a_b --> a_b : go ; ; @eq(x', x)
a_b --> a_b : tau ; @exists(y : y) ($y < 2) ; @exists(y : y) ($y < 2 ∧ $x' = $y)
@enduml
`,
		},
		{
			name: "the vending machine and its user, as examples/compose records them",
			args: []string{"-sync", "insert;showAvailable;showPurchasable;choose;drop",
				"../../../examples/valid/user.puml", "../../../examples/valid/vending_machine.puml"},
			want: tools.Untitled(recorded(t, "../../../examples/compose/user_vending_machine.puml")),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()

			// Execute
			exitStatus := cmdFunc(c.args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Fatalf("want 0, got %d", exitStatus)
			}
			if diff := cmp.Diff(c.want, tools.Untitled(spy.Stdout.String())); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncTitlesTheDiagramWithItsCommandLine(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	want := "@startuml auto-generated-by: csdfparallel -sync sync ../../../examples/valid/in.puml ../../../examples/valid/out.puml\n"

	// Execute
	cmdFunc([]string{"-sync", "sync", "../../../examples/valid/in.puml", "../../../examples/valid/out.puml"}, spy.New())

	// Assert
	if !strings.HasPrefix(spy.Stdout.String(), want) {
		t.Errorf("want %q first, got %q", want, spy.Stdout.String())
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
