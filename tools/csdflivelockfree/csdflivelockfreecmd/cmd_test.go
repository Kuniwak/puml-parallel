package csdflivelockfreecmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// runAndDecode runs the command and decodes stdout as an ObligationIR, asserting a
// zero exit status and empty stderr.
func runAndDecode(t *testing.T, spy *cli.ProcInoutSpy, args []string) obligationir.ObligationIR {
	t.Helper()
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if spy.Stderr.String() != "" {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
	var ir obligationir.ObligationIR
	if err := json.Unmarshal([]byte(spy.Stdout.String()), &ir); err != nil {
		t.Fatalf("stdout is not valid ObligationIR JSON: %v\n%s", err, spy.Stdout.String())
	}
	if ir.Goal != "livelock_free" {
		t.Errorf("goal = %q, want livelock_free", ir.Goal)
	}
	return ir
}

func TestNewMainFuncEmitsIRForStructurallyFreeDiagram(t *testing.T) {
	// Arrange: a diagram with a tau edge but no tau cycle is structurally free.
	spy := cli.SpyProcInout()

	// Act
	ir := runAndDecode(t, spy, []string{filepath.Join("testdata", "free.puml")})

	// Assert
	if !ir.StructurallyLivelockFree {
		t.Error("want structurally_livelock_free true for a tau-cycle-free diagram")
	}
}

func TestNewMainFuncEmitsIRForLivelockCandidate(t *testing.T) {
	// Arrange: user.puml has a tau self-loop on userIdle (a structural candidate).
	spy := cli.SpyProcInout()

	// Act: exits 0 and emits the obligation IR instead of signalling via exit status.
	ir := runAndDecode(t, spy, []string{"../../../examples/valid/user.puml"})

	// Assert
	if ir.StructurallyLivelockFree {
		t.Error("want structurally_livelock_free false when a reachable tau cycle exists")
	}
}

func TestNewMainFuncGuardedTauLoopExitsZero(t *testing.T) {
	// Arrange: the regression case. A tau self-loop guarded by False can never fire,
	// so it must not be reported as a definite livelock via a non-zero exit status.
	// Line 4 holds the transition, so its guard becomes Guard_L4 carrying "False".
	input := `@startuml
state "a" as a
[*] --> a
a --> a : tau ; False ; True
@enduml
`
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	// Act
	ir := runAndDecode(t, spy, []string{})

	// Assert
	if ir.StructurallyLivelockFree {
		t.Error("want structurally_livelock_free false (the tau self-loop is a candidate)")
	}
	var guard *obligationir.IRPredicate
	for i := range ir.Predicates {
		if ir.Predicates[i].Sym == "Guard_L4" {
			guard = &ir.Predicates[i]
		}
	}
	if guard == nil {
		t.Fatalf("want a Guard_L4 predicate, got %#v", ir.Predicates)
	}
	if guard.Text != "False" {
		t.Errorf("Guard_L4 text = %q, want False", guard.Text)
	}
}

func TestNewMainFuncReadsStdin(t *testing.T) {
	// Arrange: reading from stdin must be equivalent to a file argument.
	input := `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	// Act
	ir := runAndDecode(t, spy, []string{})

	// Assert
	if !ir.StructurallyLivelockFree {
		t.Error("want structurally_livelock_free true for a visible-only chain")
	}
}

func TestNewMainFuncLeanTarget(t *testing.T) {
	// -target lean compiles the obligation straight to a Lean skeleton, the same
	// output as piping the default IR through obligationirc -target lean.
	input := `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())([]string{"-target", "lean"}, spy.New())

	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if spy.Stderr.String() != "" {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"inductive St where",
		`-- "n > 0"`,
		"theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lean output missing %q\n%s", want, out)
		}
	}
}

func TestNewMainFuncIsabelleTarget(t *testing.T) {
	// -target isabelle compiles the obligation straight to an Isabelle skeleton.
	input := `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(input))

	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())([]string{"-target", "isabelle"}, spy.New())

	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if spy.Stderr.String() != "" {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"theory Livelock_Obligation imports Main begin",
		`(* "n > 0" *)`,
		`theorem livelock_free: "wf {(s', s). tau_step s s'}"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("isabelle output missing %q\n%s", want, out)
		}
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
