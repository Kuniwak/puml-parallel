package csdfsortcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncSorts(t *testing.T) {
	// Arrange: a hand-written diagram in authoring order.
	want := `@startuml
state "First" as s0
state "Second" as s1
[*] --> s1
s0 --> s1 : a
s1 --> s0 : a ; g
s1 --> s0 : b
@enduml
`
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{filepath.Join("testdata", "unsorted.puml")}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncReadsStdin(t *testing.T) {
	// Arrange: reading from stdin must be equivalent to a file argument.
	input := `@startuml
state "s1" as s1
state "s0" as s0
[*] --> s0
s0 --> s1 : a
@enduml
`
	want := `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	// Act
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncReportsParseErrors(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader("not a diagram\n"))

	// Act
	exitStatus := cmdFunc([]string{}, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Error("want non-zero exit status, got 0")
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
