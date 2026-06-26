package csdflivelockfreecmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncReportsLivelockFree(t *testing.T) {
	// Arrange: a diagram with a tau edge but no tau cycle is livelock free.
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	want := "livelock free\n"

	// Act
	exitStatus := cmdFunc([]string{filepath.Join("testdata", "free.puml")}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncDetectsLivelock(t *testing.T) {
	// Arrange: user.puml has a tau self-loop on userIdle (a livelock).
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"../../../examples/valid/user.puml"}, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Error("want non-zero exit status, got 0")
	}
	if !strings.Contains(spy.Stdout.String(), "userIdle --tau--> userIdle") {
		t.Errorf("want witness on stdout, got %q", spy.Stdout.String())
	}
	if !strings.Contains(spy.Stderr.String(), "livelock detected") {
		t.Errorf("want livelock detected on stderr, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncReadsStdin(t *testing.T) {
	// Arrange: reading from stdin must be equivalent to a file argument.
	input := `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))
	want := "livelock free\n"

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

func TestNewMainFuncVersion(t *testing.T) {
	// Arrange
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
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
