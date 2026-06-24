package csdfnormcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncNormalizes(t *testing.T) {
	// Arrange: a nondeterministic + tau diagram. The initial state is the
	// tau-closure {s0, s1}, and the two `a` edges merge into {s2, s3}.
	want := `@startuml
state "{s0, s1}" as s0_s1
state "{s2, s3}" as s2_s3
[*] --> s0_s1
s0_s1 --> s2_s3 : a
@enduml
`
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{filepath.Join("testdata", "nondeterministic.puml")}, spy.New())

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
state "s0" as s0
state "s1" as s1
state "s2" as s2
[*] --> s0
s0 --> s1 : a
s0 --> s2 : a
@enduml
`
	want := `@startuml
state "{s0}" as s0
state "{s1, s2}" as s1_s2
[*] --> s0
s0 --> s1_s2 : a
@enduml
`
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
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

func TestNewMainFuncRejectsEndEdges(t *testing.T) {
	// Arrange: end edges are not supported and must yield a clear error.
	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"../../../examples/valid/skip.puml"}, spy.New())

	// Assert
	if exitStatus == 0 {
		t.Error("want non-zero exit status, got 0")
	}
	if !strings.Contains(spy.Stderr.String(), "end edges are not supported") {
		t.Errorf("want end-edge rejection, got stderr %q", spy.Stderr.String())
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
