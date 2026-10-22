package csdflivelockfreecmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// formula is the diagram of testdata/formula named file, whose expected
// obligations are beside it.
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

func TestNewMainFuncTellsAStructuralCandidate(t *testing.T) {
	type testCase struct {
		Args  []string
		Stdin string
		Want  bool
	}

	testCases := map[string]testCase{
		"a tau edge on no cycle is structurally free": {
			Args: []string{filepath.Join("testdata", "free.puml")},
			Want: true,
		},
		"a reachable tau self-loop is a candidate": {
			Args: []string{"../../../examples/valid/user.puml"},
			Want: false,
		},
		"a tau self-loop whose guard is false is still a candidate, and exits 0": {
			Stdin: "@startuml\nstate \"a\" as a\na : n\n[*] --> a : @eq(n', n')\na --> a : tau ; @ne(n, n) ; @eq(n', n)\n@enduml\n",
			Want:  false,
		},
		"standard input is a file": {
			Stdin: "@startuml\nstate \"s0\" as s0\nstate \"s1\" as s1\n[*] --> s0\ns0 --> s1 : a\n@enduml\n",
			Want:  true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			exitStatus, spy := run(tc.Stdin, tc.Args)

			// Assert
			if exitStatus != 0 || spy.Stderr.String() != "" {
				t.Fatalf("exit %d, stderr %q; want 0 and nothing", exitStatus, spy.Stderr.String())
			}
			var ir obligationir.LivelockFree
			if err := json.Unmarshal([]byte(spy.Stdout.String()), &ir); err != nil {
				t.Fatalf("stdout is not valid obligation IR JSON: %v\n%s", err, spy.Stdout.String())
			}
			if ir.Structurally != tc.Want {
				t.Errorf("structurally = %v, want %v", ir.Structurally, tc.Want)
			}
		})
	}
}

// TestNewMainFuncCompilesToTheTarget holds -target to the obligation
// expected of the diagram in that format.
func TestNewMainFuncCompilesToTheTarget(t *testing.T) {
	for name, ext := range map[string]string{"ir-json": ".json", "isabelle": ".thy", "lean": ".lean"} {
		t.Run(name, func(t *testing.T) {
			// Setup
			want, err := os.ReadFile(formula("countdown" + ext))
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			exitStatus, spy := run("", []string{"-target", name, formula("countdown.puml")})

			// Assert
			if exitStatus != 0 {
				t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
			}
			if diff := cmp.Diff(string(want), spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
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
