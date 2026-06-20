package csdfparsecmd

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncPrintsJSON(t *testing.T) {
	// Arrange
	input := `@startuml
state "Initial" as s0
s0: ready ; bool
s0: count
state "Done" as s1
[*] --> s0 : initialize
s0 --> s1 : finish(result) ; ready ; done
s1 --> [*] : complete
@enduml
`
	want := `{"states":{"s0":{"id":"s0","name":"Initial","vars":[{"name":"ready","type":"bool"},{"name":"count"}]},"s1":{"id":"s1","name":"Done","vars":[]}},"start_edge":{"dst":"s0","post":"initialize"},"edges":[{"src":"s0","dst":"s1","event":"finish(result)","guard":"ready","post":"done"}],"end_edge":{"src":"s1","guard":"complete"}}` + "\n"

	cmdFunc := cli.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout(input)

	// Act
	exitStatus := cmdFunc([]string{}, spy.NewProcInout())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
	if spy.Stderr.Len() != 0 {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
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
