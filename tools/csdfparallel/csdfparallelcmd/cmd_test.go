package csdfparallelcmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncCompose(t *testing.T) {
	// Arrange
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	want := `@startuml
state "s0 || s0" as s0_s0
state "s1 || s0" as s1_s0
state "s2 || s1" as s2_s1
state "s2 || s2" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : sync
s2_s1 --> s2_s2 : out
@enduml
`

	// Act
	exitStatus := cmdFunc([]string{
		"-sync", "sync",
		"../../../examples/valid/in.puml",
		"../../../examples/valid/out.puml",
	}, spy.NewProcInout())

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
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"-v"}, spy.NewProcInout())

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
