package csdftracechkcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func TestNewMainFunc(t *testing.T) {
	type testCase struct {
		Args       []string
		WantExit   int
		WantStdout []string // substrings
		WantStderr []string // substrings
	}

	diagram := filepath.Join("testdata", "a.puml")
	ok := filepath.Join("testdata", "ok.tsv")
	ng := filepath.Join("testdata", "ng.tsv")
	stripped := filepath.Join("testdata", "stripped.tsv")

	testCases := map[string]testCase{
		"an accepted trace exits 0 with the obligations": {
			Args:     []string{diagram, ok},
			WantExit: 0,
			WantStdout: []string{
				"# " + ok + ": ACCEPTED",
				"coins' is {coin}",
				"Obligation: ∀ x0 x1. post_1(x0, x1) → true",
				"## Prompt",
			},
		},
		"a rejected trace exits 1 and says where": {
			Args:     []string{diagram, ng},
			WantExit: 1,
			WantStdout: []string{
				": REJECTED",
				"After the empty prefix (0 of 1 events) the diagram may be in a.",
				"a is stable and has no edge for `reset`",
				"visible events enabled at the refusing states: `insert(coin)`",
			},
		},
		"every trace is reported and any rejection fails the run": {
			Args:       []string{diagram, ok, ng},
			WantExit:   1,
			WantStdout: []string{": ACCEPTED", ": REJECTED"},
		},
		"-match prefix accepts a trace stripped of its parameters": {
			Args:     []string{"-match", "prefix", diagram, stripped},
			WantExit: 0,
		},
		"-match exact rejects a trace stripped of its parameters": {
			Args:     []string{diagram, stripped},
			WantExit: 1,
		},
		"a malformed trace is an error naming the file": {
			Args:       []string{diagram, diagram},
			WantExit:   1,
			WantStderr: []string{"a.puml"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			spy := cli.SpyProcInout()

			// Act
			exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(testCase.Args, spy.New())

			// Assert
			if exitStatus != testCase.WantExit {
				t.Fatalf("want exit %d, got %d (stdout: %s; stderr: %s)", testCase.WantExit, exitStatus, spy.Stdout.String(), spy.Stderr.String())
			}
			for _, want := range testCase.WantStdout {
				if !strings.Contains(spy.Stdout.String(), want) {
					t.Errorf("stdout missing %q\n%s", want, spy.Stdout.String())
				}
			}
			for _, want := range testCase.WantStderr {
				if !strings.Contains(spy.Stderr.String(), want) {
					t.Errorf("stderr missing %q\n%s", want, spy.Stderr.String())
				}
			}
		})
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Arrange
	spy := cli.SpyProcInout()

	// Act
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())([]string{"-v"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(version.Version+"\n", spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}
