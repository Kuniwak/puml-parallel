package csdfrefinementcmd

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

func run(t *testing.T, spy *cli.ProcInoutSpy, args []string) string {
	t.Helper()
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	if spy.Stderr.String() != "" {
		t.Errorf("want empty stderr, got %q", spy.Stderr.String())
	}
	return spy.Stdout.String()
}

func TestNewMainFuncEmitsIR(t *testing.T) {
	// Arrange: two diagrams over overlapping alphabets.
	spy := cli.SpyProcInout()

	// Act
	out := run(t, spy, []string{
		"-m", "t",
		filepath.Join("testdata", "spec.puml"),
		filepath.Join("testdata", "impl.puml"),
	})

	// Assert
	var ir obligationir.IRRefinement
	if err := json.Unmarshal([]byte(out), &ir); err != nil {
		t.Fatalf("stdout is not valid obligation IR JSON: %v\n%s", err, out)
	}
	if ir.Mode != obligationir.IRRefinementModeTrace {
		t.Errorf("mode = %q, want trace", ir.Mode)
	}
	if diff := cmp.Diff([]string{"a"}, []string{string(ir.Alphabet[0])}); diff != "" {
		t.Error(diff)
	}
	if ir.Spec.Init.Dst != "s0" || ir.Impl.Init.Dst != "t0" {
		t.Errorf("want spec s0 and impl t0, got %q and %q", ir.Spec.Init.Dst, ir.Impl.Init.Dst)
	}
	// The order of the arguments is the direction of the obligation, so it must
	// not be symmetric: the abstract diagram comes first.
	if ir.Spec.Init.Dst == ir.Impl.Init.Dst {
		t.Error("want the two sides to stay distinct")
	}
}

func TestNewMainFuncFailuresDivergenceCarriesLivelockFields(t *testing.T) {
	spy := cli.SpyProcInout()

	out := run(t, spy, []string{
		"-m", "fd",
		filepath.Join("testdata", "spec.puml"),
		filepath.Join("testdata", "impl.puml"),
	})

	var ir obligationir.IRRefinement
	if err := json.Unmarshal([]byte(out), &ir); err != nil {
		t.Fatalf("stdout is not valid obligation IR JSON: %v\n%s", err, out)
	}
	if ir.Spec.StructurallyLivelockFree == nil || !*ir.Spec.StructurallyLivelockFree {
		t.Errorf("want a tau-free spec to be structurally livelock free, got %#v", ir.Spec.StructurallyLivelockFree)
	}
}

func TestNewMainFuncReadsOneDiagramFromStdin(t *testing.T) {
	// Arrange: at most one side may come from stdin, the other from a file.
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : a
@enduml
`))

	// Act
	out := run(t, spy, []string{"-m", "f", "-", filepath.Join("testdata", "impl.puml")})

	// Assert
	var ir obligationir.IRRefinement
	if err := json.Unmarshal([]byte(out), &ir); err != nil {
		t.Fatalf("stdout is not valid obligation IR JSON: %v\n%s", err, out)
	}
	if ir.Spec.Init.Dst != "s0" {
		t.Errorf("want the spec read from stdin, got %q", ir.Spec.Init.Dst)
	}
}

func TestNewMainFuncIsabelleTarget(t *testing.T) {
	spy := cli.SpyProcInout()

	out := run(t, spy, []string{
		"-m", "fd", "-target", "isabelle",
		filepath.Join("testdata", "spec.puml"),
		filepath.Join("testdata", "impl.puml"),
	})

	for _, want := range []string{
		"theory Refinement_Obligation\n  imports CSP_F.CSP_F",
		`overloading Set_procfun == "PNfun :: (PN, event) pnfun"`,
		`theorem refines_f: "SpecProc <=F ImplProc"`,
		// Neither diagram has a tau edge, so both discharge divergence freedom
		// structurally and no wf_on obligation is emitted.
		"(* Spec is livelock free structurally: no reachable tau-cycle. *)",
		"(* Impl is livelock free structurally: no reachable tau-cycle. *)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("isabelle output missing %q\n%s", want, out)
		}
	}
}

func TestNewMainFuncRejectsLeanTarget(t *testing.T) {
	// Arrange: lean is deferred until the in-house Lean translation of CSP-Prover
	// is published, and must say so rather than fail as an unknown target.
	spy := cli.SpyProcInout()

	// Act
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())([]string{
		"-m", "t", "-target", "lean",
		filepath.Join("testdata", "spec.puml"),
		filepath.Join("testdata", "impl.puml"),
	}, spy.New())

	// Assert
	if exitStatus != 1 {
		t.Errorf("want exit 1, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stderr.String(), "CSP-Prover") {
		t.Errorf("want the deferral reason on stderr, got %q", spy.Stderr.String())
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
