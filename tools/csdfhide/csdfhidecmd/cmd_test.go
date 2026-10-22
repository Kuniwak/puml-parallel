package csdfhidecmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// TestNewMainFuncHideBindsAReceivedField hides an event whose field the
// guard and post read: the field is bound by ∃ in both.
func TestNewMainFuncHideBindsAReceivedField(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	want := `@startuml
state "a" as a
a : x
[*] --> a : $x' = 0
a --> a : tau ; @exists(k : k) ($k < 2) ; @exists(k : k) ($k < 2 ∧ $x' = $k)
@enduml
`

	// Execute
	exitStatus := cmdFunc([]string{"-channels", "put", "testdata/put.puml"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncHide(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	want := `@startuml
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : in
s1 --> s2 : tau
@enduml
`

	// Act
	exitStatus := cmdFunc([]string{"-channels", "sync", "../../../examples/valid/in.puml"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
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
