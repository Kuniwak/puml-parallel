package csdfeventscmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncOnlyCommon(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	want := "choose\ndrop\ninsert\nshowAvailable\nshowPurchasable\n"

	// Act
	exitStatus := cmdFunc([]string{
		"-only-common",
		"../../../examples/valid/user.puml",
		"../../../examples/valid/vending_machine.puml",
	}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncBuiltins(t *testing.T) {
	type testCase struct {
		Args []string
		Want string
	}

	testCases := map[string]testCase{
		"all channels of a diagram whose clauses use the builtins": {
			Args: []string{filepath.Join("testdata", "builtins.puml")},
			Want: "add\nclose\nopen\n",
		},
		"all channels of two diagrams": {
			Args: []string{filepath.Join("testdata", "builtins.puml"), filepath.Join("testdata", "door.puml")},
			Want: "add\nclose\nopen\ntau\n",
		},
		"only the channels common to two diagrams": {
			Args: []string{"-only-common", filepath.Join("testdata", "builtins.puml"), filepath.Join("testdata", "door.puml")},
			Want: "close\nopen\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()

			// Act
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Errorf("want 0, got %d", exitStatus)
			}
			if diff := cmp.Diff(testCase.Want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncRefusesOldClauseSyntax(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{filepath.Join("testdata", "old_opaque.puml")}, spy.New())

	// Assert
	if exitStatus != 1 {
		t.Errorf("want 1, got %d", exitStatus)
	}
	if want := "unknown tool word @opaque"; !strings.Contains(spy.Stderr.String(), want) {
		t.Errorf("want %q on stderr, got %q", want, spy.Stderr.String())
	}
	if spy.Stdout.String() != "" {
		t.Errorf("want nothing on stdout, got %q", spy.Stdout.String())
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
