package obligationirccmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/version"
	"github.com/google/go-cmp/cmp"
)

// formula is the file of testdata/formula named file: the IR of a diagram
// and the obligations expected of it.
func formula(file string) string {
	return filepath.Join("..", "..", "..", "csdf", "obligationir", "testdata", "formula", file)
}

func run(stdin string, args []string) (int, *cli.ProcInoutSpy) {
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(stdin))
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	return exitStatus, spy
}

// TestNewMainFuncCompilesTheIR holds obligationirc to the obligation expected
// of the IR in each format, which is what csdflivelockfree -target writes, so
// csdflivelockfree | obligationirc is csdflivelockfree -target.
func TestNewMainFuncCompilesTheIR(t *testing.T) {
	for name, ext := range map[string]string{"ir-json": ".json", "isabelle": ".thy", "lean": ".lean"} {
		t.Run(name, func(t *testing.T) {
			// Setup
			want, err := os.ReadFile(formula("map" + ext))
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			exitStatus, spy := run("", []string{"-target", name, formula("map.json")})

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

func TestNewMainFuncRefusesAnIRItDoesNotRead(t *testing.T) {
	testCases := map[string]string{
		"an unknown field": `{"version":2,"structurally":true,"states":[{"name":"a","vars":[]}],"init":"a","init_post":{"op":"true"},"edges":[],"lemmas":[],"assumptions":[],"predicates":{}}`,
		"another version":  `{"version":0,"structurally":true,"states":[{"name":"a","vars":[]}],"init":"a","init_post":{"op":"true"},"edges":[],"lemmas":[],"assumptions":[]}`,
		"an unknown state": `{"version":2,"structurally":false,"states":[{"name":"a","vars":[]}],"init":"b","init_post":{"op":"true"},"edges":[],"lemmas":[],"assumptions":[]}`,
		"an unknown op":    `{"version":2,"structurally":false,"states":[{"name":"a","vars":[]}],"init":"a","init_post":{"op":"xor"},"edges":[],"lemmas":[],"assumptions":[]}`,
	}

	for name, input := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			exitStatus, spy := run(input, []string{"-target", "ir-json"})

			// Assert
			if exitStatus == 0 {
				t.Errorf("want non-zero exit, got 0\nstdout: %s", spy.Stdout.String())
			}
		})
	}
}

func TestNewMainFuncInvalidJSONFails(t *testing.T) {
	// Non-JSON input is a hard error (exit 1), unlike opaque-predicate verdicts.
	exitStatus, spy := run("not json\n", []string{"-target", "lean"})
	if exitStatus == 0 {
		t.Errorf("want non-zero exit for invalid IR JSON, got 0\nstdout: %s", spy.Stdout.String())
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Arrange / Act
	exitStatus, spy := run("", []string{"-v"})

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
