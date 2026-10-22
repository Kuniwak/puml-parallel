package csdflivelockfreecmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

const countdown = `@startuml
state "a" as a
a : n
[*] --> a : $n' = 10
a --> a : tau ; $n > 0 ; $n' = $n - 1
@enduml
`

// run runs the command, asserting a zero exit status and empty stderr.
func run(t *testing.T, stdin string, args []string) string {
	t.Helper()
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(stdin))
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if spy.Stderr.String() != "" {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
	return spy.Stdout.String()
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
			Stdin: "@startuml\nstate \"a\" as a\n[*] --> a\na --> a : tau ; 偽 ; \n@enduml\n",
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
			out := run(t, tc.Stdin, tc.Args)

			// Assert
			var ir obligationir.LivelockFree
			if err := json.Unmarshal([]byte(out), &ir); err != nil {
				t.Fatalf("stdout is not valid obligation IR JSON: %v\n%s", err, out)
			}
			if ir.Structurally != tc.Want {
				t.Errorf("structurally = %v, want %v", ir.Structurally, tc.Want)
			}
		})
	}
}

// TestNewMainFuncCompilesToTheTarget holds -target to the back end it names.
func TestNewMainFuncCompilesToTheTarget(t *testing.T) {
	for _, name := range []target.Name{target.NameIRJSON, target.NameIsabelle, target.NameLean} {
		t.Run(string(name), func(t *testing.T) {
			// Setup
			d, err := coreio.Read(countdown)
			if err != nil {
				t.Fatal(err)
			}
			ir, err := obligationir.BuildFormulaLivelockFree(d)
			if err != nil {
				t.Fatal(err)
			}
			var want strings.Builder
			if err := target.Compile(&want, ir, name); err != nil {
				t.Fatal(err)
			}

			// Execute
			got := run(t, countdown, []string{"-target", string(name)})

			// Assert
			if diff := cmp.Diff(want.String(), got); diff != "" {
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
