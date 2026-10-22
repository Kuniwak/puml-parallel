package obligationirccmd

import (
	"encoding/json"
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

// canonicalIR builds the obligation IR for a guarded tau self-loop, the kind
// of input obligationirc consumes from csdflivelockfree, and its JSON.
func canonicalIR(t *testing.T) (obligationir.LivelockFree, string) {
	t.Helper()
	d, err := coreio.Read(countdown)
	if err != nil {
		t.Fatalf("coreio.Read() error = %v", err)
	}
	ir, err := obligationir.BuildFormulaLivelockFree(d)
	if err != nil {
		t.Fatalf("BuildFormulaLivelockFree() error = %v", err)
	}
	bs, err := json.Marshal(ir)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return ir, string(bs)
}

func run(t *testing.T, stdin string, args []string) (int, *cli.ProcInoutSpy) {
	t.Helper()
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(stdin))
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	return exitStatus, spy
}

// TestNewMainFuncCompilesAsTheTargetDoes holds obligationirc to what the
// target compiles the IR to directly, so csdflivelockfree | obligationirc is
// csdflivelockfree -target.
func TestNewMainFuncCompilesAsTheTargetDoes(t *testing.T) {
	for _, name := range []target.Name{target.NameIRJSON, target.NameIsabelle, target.NameLean} {
		t.Run(string(name), func(t *testing.T) {
			// Setup
			ir, input := canonicalIR(t)
			var want strings.Builder
			if err := target.Compile(&want, ir, name); err != nil {
				t.Fatal(err)
			}

			// Execute
			exitStatus, spy := run(t, input, []string{"-target", string(name)})

			// Assert
			if exitStatus != 0 {
				t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
			}
			if diff := cmp.Diff(want.String(), spy.Stdout.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNewMainFuncInvalidJSONFails(t *testing.T) {
	// Non-JSON input is a hard error (exit 1), unlike opaque-predicate verdicts.
	exitStatus, spy := run(t, "not json\n", []string{"-target", "lean"})
	if exitStatus == 0 {
		t.Errorf("want non-zero exit for invalid IR JSON, got 0\nstdout: %s", spy.Stdout.String())
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Arrange / Act
	exitStatus, spy := run(t, "", []string{"-v"})

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
