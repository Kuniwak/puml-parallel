package csdfcompcmd

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

const composed = `@startuml
state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
state "(s2, s1)" as s2_s1
state "(s2, s2)" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : tau
s2_s1 --> s2_s2 : out
@enduml
`

func TestNewMainFuncComposesATreeFile(t *testing.T) {
	// Arrange: paths in the tree are resolved against the directory of the tree file.
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"../../../examples/valid/in_out_tree.json"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(composed, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncComposesATreeFromStdinWithBaseDir(t *testing.T) {
	// Arrange: a tree read from stdin resolves paths against -base.
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = strings.NewReader(`{
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
	}`)

	// Act
	exitStatus := cmdFunc([]string{"-base", "../../../examples/valid", "-"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(composed, spy.Stdout.String()); diff != "" {
		t.Error(diff)
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
