package csdfeventscmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFuncOK(t *testing.T) {
	type testCase struct {
		Args []string
		Want string
	}

	testCases := map[string]testCase{
		"only the channels common to two example diagrams": {
			Args: []string{"-only-common", "../../../examples/valid/user.puml", "../../../examples/valid/vending_machine.puml"},
			Want: "choose\ndrop\ninsert\nshowAvailable\nshowPurchasable\n",
		},
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
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()

			// Execute
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

func TestNewMainFuncRefuses(t *testing.T) {
	type testCase struct {
		Args       []string
		WantStderr string
	}

	testCases := map[string]testCase{
		"@opaque": {
			Args:       []string{filepath.Join("testdata", "old_opaque.puml")},
			WantStderr: "unknown tool word @opaque",
		},
		"a file that is missing": {
			Args:       []string{filepath.Join("testdata", "missing.puml")},
			WantStderr: "no such file or directory",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()

			// Execute
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			if exitStatus != 1 {
				t.Errorf("want 1, got %d", exitStatus)
			}
			if !strings.Contains(spy.Stderr.String(), testCase.WantStderr) {
				t.Errorf("want %q on stderr, got %q", testCase.WantStderr, spy.Stderr.String())
			}
			if spy.Stdout.String() != "" {
				t.Errorf("want nothing on stdout, got %q", spy.Stdout.String())
			}
		})
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Execute
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
