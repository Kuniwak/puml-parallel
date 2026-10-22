package csdfrefinementcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// formula is the file of testdata/formula named file: a diagram, or the
// obligations expected of a pair of them under refinement/.
func formula(file string) string {
	return filepath.Join("..", "..", "..", "csdf", "obligationir", "testdata", "formula", file)
}

// run runs the command on args and stdin.
func run(stdin string, args []string) (int, *cli.ProcInoutSpy) {
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(stdin))
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	return exitStatus, spy
}

// TestNewMainFuncCompilesToTheTarget holds -m and -target to the obligation
// expected of the pair, the abstract diagram first.
func TestNewMainFuncCompilesToTheTarget(t *testing.T) {
	type testCase struct {
		Args  []string
		Stdin string
		Want  string
	}

	testCases := map[string]testCase{
		"traces, as JSON": {
			Args: []string{"-m", "t", formula("counter.puml"), formula("countdown.puml")},
			Want: "refinement/counter_countdown.t.json",
		},
		"stable failures, for Isabelle": {
			Args: []string{"-m", "f", "-target", "isabelle", formula("counter.puml"), formula("counter.puml")},
			Want: "refinement/counter_counter.f.thy",
		},
		"failures-divergences, for Lean, the impl from standard input": {
			Args:  []string{"-m", "fd", "-target", "lean", formula("map.puml"), "-"},
			Stdin: mustRead(formula("countdown.puml")),
			Want:  "refinement/map_countdown.fd.lean",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			want := mustRead(formula(tc.Want))

			// Execute
			exitStatus, spy := run(tc.Stdin, tc.Args)

			// Assert
			if exitStatus != 0 {
				t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
			}
			if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncRefusesAChannelOfTwoArities(t *testing.T) {
	// Setup: inc carries one field in the spec and none in the impl.
	impl := "@startuml\nstate \"a\" as a\n[*] --> a\na --> a : inc\n@enduml\n"

	// Execute
	exitStatus, spy := run(impl, []string{"-m", "t", formula("counter.puml"), "-"})

	// Assert
	if exitStatus == 0 || !strings.Contains(spy.Stderr.String(), "the channel inc has") {
		t.Errorf("exit %d, stderr %q; want the channel refused", exitStatus, spy.Stderr.String())
	}
}

func mustRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(b)
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
