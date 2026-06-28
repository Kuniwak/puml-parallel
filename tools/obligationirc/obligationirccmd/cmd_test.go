package obligationirccmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// canonicalIRJSON builds the obligation IR for a guarded tau self-loop and encodes it
// as JSON, the kind of input obligationirc consumes from csdflivelockfree.
func canonicalIRJSON(t *testing.T) string {
	t.Helper()
	d, err := csdf.ParseDiagram([]byte(`@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	bs, err := json.Marshal(obligationir.BuildObligationIR(d))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return string(bs)
}

func run(t *testing.T, stdin string, args []string) (int, *cli.ProcInoutSpy) {
	t.Helper()
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(stdin))
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	return exitStatus, spy
}

func TestNewMainFuncDefaultTargetReemitsIR(t *testing.T) {
	// The default target re-emits valid obligation IR JSON.
	exitStatus, spy := run(t, canonicalIRJSON(t), []string{})
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	var ir obligationir.ObligationIR
	if err := json.Unmarshal([]byte(spy.Stdout.String()), &ir); err != nil {
		t.Fatalf("stdout is not valid ObligationIR JSON: %v\n%s", err, spy.Stdout.String())
	}
	if ir.Goal != "livelock_free" {
		t.Errorf("goal = %q, want livelock_free", ir.Goal)
	}
}

func TestNewMainFuncLeanTarget(t *testing.T) {
	// The lean target emits the theorem, the placeholder def, and the NL comment.
	exitStatus, spy := run(t, canonicalIRJSON(t), []string{"-target", "lean"})
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"inductive St where",
		`-- "n > 0"`,
		"def Guard_L5 (n : Nat) : Prop := True",
		"theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lean output missing %q\n%s", want, out)
		}
	}
}

func TestNewMainFuncIsabelleTarget(t *testing.T) {
	// The isabelle target emits the theory, the placeholder definition, and the NL comment.
	exitStatus, spy := run(t, canonicalIRJSON(t), []string{"-target", "isabelle"})
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"theory Livelock_Obligation imports Main begin",
		`(* "n > 0" *)`,
		`definition Guard_L5 :: "nat ⇒ bool" where "Guard_L5 n ≡ True"`,
		`theorem livelock_free: "wf {(s', s). tau_step s s'}"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("isabelle output missing %q\n%s", want, out)
		}
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
