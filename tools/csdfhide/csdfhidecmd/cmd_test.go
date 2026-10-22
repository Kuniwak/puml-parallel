package csdfhidecmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncHide(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "an event without fields becomes tau",
			args: []string{"-channels", "sync", "../../../examples/valid/in.puml"},
			want: `@startuml
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : in
s1 --> s2 : tau
@enduml
`,
		},
		{
			name: "a field the guard and post read is bound by ∃ in both",
			args: []string{"-channels", "put", "testdata/put.puml"},
			want: `@startuml
state "a" as a
a : x
[*] --> a : $x' = 0
a --> a : tau ; @exists(k : k) ($k < 2) ; @exists(k : k) ($k < 2 ∧ $x' = $k)
@enduml
`,
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
			if diff := cmp.Diff(c.want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
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
