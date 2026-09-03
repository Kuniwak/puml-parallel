// Package provercheck runs generated proof obligations through the provers
// themselves.
//
// The golden tests in the backend packages compare strings, so they cannot tell
// a well-formed theory from one that does not parse - and they let several
// through: an unparenthesised replicated internal choice, an unparenthesised
// prefix in an IF branch, and encoded names ending in "_", each of which made
// every affected obligation unusable. Completeness starts here: a diagram that
// refines itself must at least yield a theory the prover accepts.
//
// The provers are not vendored, so each check is skipped unless its library is
// pointed at by an environment variable:
//
//	CSDF_LEAN_CSP_PROVER  a built github.com/Kuniwak/lean-csp-prover checkout
//	CSDF_CSP_PROVER       a github.com/Kuniwak/CSP-Prover checkout (Isabelle2025 fork)
//	CSDF_ISABELLE         the isabelle executable (default: "isabelle" on PATH)
//
// Set CSDF_REQUIRE_PROVERS=1 to turn a skip into a failure, so that CI cannot
// pass by checking nothing.
package provercheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/lean"
)

const (
	envLean     = "CSDF_LEAN_CSP_PROVER"
	envCSP      = "CSDF_CSP_PROVER"
	envIsabelle = "CSDF_ISABELLE"
	envRequire  = "CSDF_REQUIRE_PROVERS"
)

// proverTimeout bounds one prover invocation. Isabelle takes a few seconds per
// session and Lean a few seconds per file, so this is slack, not a budget.
const proverTimeout = 15 * time.Minute

// obligation is one generated artifact together with what the prover needs to
// know about it: the theory name Isabelle requires the file to be named after,
// and the session its ROOT has to extend.
type obligation struct {
	name    string
	theory  string
	parent  string
	leanSrc string
	isaSrc  string
}

// livelockObligation builds the livelock-freedom obligation for one diagram.
func livelockObligation(t *testing.T, name, path string) obligation {
	t.Helper()
	d := parse(t, path)
	ir := obligationir.BuildLivelockFree(d)

	var leanBuf, isaBuf strings.Builder
	if err := lean.WriteLivelockFree(&leanBuf, ir); err != nil {
		t.Fatalf("lean.WriteLivelockFree(%s) error = %v", name, err)
	}
	if err := isabelle.WriteLivelockFree(&isaBuf, ir); err != nil {
		t.Fatalf("isabelle.WriteLivelockFree(%s) error = %v", name, err)
	}
	return obligation{
		name:    name,
		theory:  "Livelock_Obligation",
		parent:  "HOL",
		leanSrc: leanBuf.String(),
		isaSrc:  isaBuf.String(),
	}
}

// refinementObligation builds the refinement obligation for one pair of diagrams
// in one model.
func refinementObligation(t *testing.T, name string, mode obligationir.IRRefinementMode, specPath, implPath string) obligation {
	t.Helper()
	ir := obligationir.BuildRefinement(mode, parse(t, specPath), parse(t, implPath))

	var leanBuf, isaBuf strings.Builder
	if err := lean.WriteRefinement(&leanBuf, ir); err != nil {
		t.Fatalf("lean.WriteRefinement(%s) error = %v", name, err)
	}
	if err := isabelle.WriteRefinement(&isaBuf, ir); err != nil {
		t.Fatalf("isabelle.WriteRefinement(%s) error = %v", name, err)
	}

	// The T model lives in its own CSP-Prover session; F backs both f and fd.
	parent := "CSP_F"
	if mode == obligationir.IRRefinementModeTrace {
		parent = "CSP_T"
	}
	return obligation{
		name:    name,
		theory:  "Refinement_Obligation",
		parent:  parent,
		leanSrc: leanBuf.String(),
		isaSrc:  isaBuf.String(),
	}
}

func parse(t *testing.T, path string) *csdf.Diagram {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	d, err := csdf.ParseBytes(src)
	if err != nil {
		t.Fatalf("ParseBytes(%q) error = %v", path, err)
	}
	return d
}

// obligations is the corpus both provers are held to. Between them the cases
// cover every construct of the encoding: state variables and the replicated
// internal choice they force, tau and its hiding, an end edge's guarded SKIP, a
// side that is structurally livelock free and one that is not, and names that are
// legal CSDF but not identifiers.
func obligations(t *testing.T) []obligation {
	t.Helper()
	const (
		hostileSpec = "testdata/hostile_spec.puml"
		hostileImpl = "testdata/hostile_impl.puml"
		groundSpec  = "../../../examples/refinement_spec.puml"
		groundImpl  = "../../../examples/refinement_impl.puml"
		livelock    = "../../../examples/livelock.puml"
	)
	return []obligation{
		livelockObligation(t, "livelock_example", livelock),
		livelockObligation(t, "livelock_hostile_names", hostileSpec),
		refinementObligation(t, "refinement_ground_f", obligationir.IRRefinementModeStableFailure, groundSpec, groundImpl),
		refinementObligation(t, "refinement_hostile_t", obligationir.IRRefinementModeTrace, hostileSpec, hostileImpl),
		refinementObligation(t, "refinement_hostile_f", obligationir.IRRefinementModeStableFailure, hostileSpec, hostileImpl),
		refinementObligation(t, "refinement_hostile_fd", obligationir.IRRefinementModeFailuresDivergence, hostileSpec, hostileImpl),
		// A diagram compared with itself refines itself, so this is the plainest
		// completeness case there is: it must at least be checkable.
		refinementObligation(t, "refinement_reflexive_f", obligationir.IRRefinementModeStableFailure, hostileSpec, hostileSpec),
	}
}

func TestLeanAcceptsTheGeneratedObligations(t *testing.T) {
	leanDir := requireEnv(t, envLean)
	for _, o := range obligations(t) {
		t.Run(o.name, func(t *testing.T) {
			out, err := runLean(t, leanDir, o.name+".lean", o.leanSrc)
			if err != nil {
				t.Fatalf("lean rejected the obligation: %v\n%s\n--- source ---\n%s", err, out, o.leanSrc)
			}
			if diags := unexpectedLeanDiagnostics(out); len(diags) > 0 {
				t.Errorf("lean reported %v\n--- source ---\n%s", diags, o.leanSrc)
			}
		})
	}
}

func TestIsabelleAcceptsTheGeneratedObligations(t *testing.T) {
	cspDir := requireEnv(t, envCSP)
	for _, o := range obligations(t) {
		t.Run(o.name, func(t *testing.T) {
			out, err := runIsabelle(t, cspDir, o, o.isaSrc)
			if err != nil {
				t.Fatalf("isabelle rejected the obligation: %v\n%s\n--- source ---\n%s", err, out, o.isaSrc)
			}
		})
	}
}

// TestLeanAcceptsTheShippedArtifacts guards the checked-in obligations, whose
// predicate bodies and proofs are filled in by hand and so drift out of step with
// the generator silently.
func TestLeanAcceptsTheShippedArtifacts(t *testing.T) {
	leanDir := requireEnv(t, envLean)
	for _, path := range []string{
		"../../../examples/Livelock_Obligation.lean",
		"../../../examples/Refinement_Obligation.lean",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			out, err := runLean(t, leanDir, filepath.Base(path), string(src))
			if err != nil {
				t.Fatalf("lean rejected %s: %v\n%s", path, err, out)
			}
			if diags := unexpectedLeanDiagnostics(out); len(diags) > 0 {
				t.Errorf("lean reported %v on %s", diags, path)
			}
		})
	}
}

func TestIsabelleAcceptsTheShippedArtifacts(t *testing.T) {
	cspDir := requireEnv(t, envCSP)
	for _, tt := range []struct {
		path   string
		theory string
		parent string
	}{
		{path: "../../../examples/Livelock_Obligation.thy", theory: "Livelock_Obligation", parent: "HOL"},
		{path: "../../../examples/Refinement_Obligation.thy", theory: "Refinement_Obligation", parent: "CSP_F"},
	} {
		t.Run(filepath.Base(tt.path), func(t *testing.T) {
			src, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", tt.path, err)
			}
			o := obligation{name: "shipped_" + tt.theory, theory: tt.theory, parent: tt.parent}
			if out, err := runIsabelle(t, cspDir, o, string(src)); err != nil {
				t.Fatalf("isabelle rejected %s: %v\n%s", tt.path, err, out)
			}
		})
	}
}

// TestLeanRejectsABrokenObligation is the negative control for the Lean checks:
// without it a misconfigured runner that never really invokes Lean would report a
// clean pass.
func TestLeanRejectsABrokenObligation(t *testing.T) {
	leanDir := requireEnv(t, envLean)
	o := obligations(t)[0]
	broken := strings.Replace(o.leanSrc, "inductive St where", "inductive St where BROKEN(", 1)
	if broken == o.leanSrc {
		t.Fatal("the obligation no longer contains the text this control breaks")
	}
	if out, err := runLean(t, leanDir, "broken.lean", broken); err == nil {
		t.Errorf("lean accepted a deliberately broken obligation\n%s", out)
	}
}

func TestIsabelleRejectsABrokenObligation(t *testing.T) {
	cspDir := requireEnv(t, envCSP)
	o := obligations(t)[0]
	broken := strings.Replace(o.isaSrc, "datatype st =", "datatype st = BROKEN(", 1)
	if broken == o.isaSrc {
		t.Fatal("the obligation no longer contains the text this control breaks")
	}
	if out, err := runIsabelle(t, cspDir, o, broken); err == nil {
		t.Errorf("isabelle accepted a deliberately broken obligation\n%s", out)
	}
}

// TestLeanCannotDischargeAnUnformalisedPredicate is the soundness control. The
// Impl's only "a" edge is guarded by a predicate nobody has formalised, so
// nothing about its enabledness may be provable. Defining the predicate as True
// instead - as the generator once did - makes it provable by trivial, which is
// the second half of this test: it shows the probe has teeth rather than failing
// for some unrelated reason.
func TestLeanCannotDischargeAnUnformalisedPredicate(t *testing.T) {
	leanDir := requireEnv(t, envLean)
	o := refinementObligation(t, "disabled", obligationir.IRRefinementModeStableFailure,
		"testdata/disabled_spec.puml", "testdata/disabled_impl.puml")

	// The alias names come from the IR rather than from the fixture's line
	// numbers: a name that does not exist would be auto-bound as an implicit
	// argument and the probe would fail for that reason instead, which is exactly
	// what the True control below caught the first time round. autoImplicit is
	// switched off so that cannot happen silently again.
	ir := obligationir.BuildRefinement(obligationir.IRRefinementModeStableFailure,
		parse(t, "testdata/disabled_spec.puml"), parse(t, "testdata/disabled_impl.puml"))
	if len(ir.Impl.Edges) != 1 {
		t.Fatalf("the fixture has %d Impl edges, want exactly the one guarded edge", len(ir.Impl.Edges))
	}
	line := ir.Impl.Edges[0].Line
	probe := "\nset_option autoImplicit false in\nexample : " +
		obligationir.SideImpl.GuardName(line) + " ∧ " + obligationir.SideImpl.PostName(line) +
		" := by trivial\n"

	withProbe := strings.Replace(o.leanSrc, "end Refinement_Obligation", probe+"\nend Refinement_Obligation", 1)
	if withProbe == o.leanSrc {
		t.Fatal("could not place the probe: the namespace footer is gone")
	}

	if out, err := runLean(t, leanDir, "unformalised.lean", withProbe); err == nil {
		t.Errorf("lean discharged the enabledness of an unformalised guard\n%s", out)
	}

	// The same probe against the True-placeholder encoding the generator used to
	// emit, which is what made the obligation unsound.
	asTrue := opaquePredicate.ReplaceAllString(withProbe, "def $1 : Prop := True")
	if asTrue == withProbe {
		t.Fatal("the obligation no longer declares an opaque nullary predicate")
	}
	if out, err := runLean(t, leanDir, "unformalised_as_true.lean", asTrue); err != nil {
		t.Errorf("the control did not reproduce the True-placeholder proof, so the probe above proves nothing: %v\n%s", err, out)
	}
}

// opaquePredicate matches the nullary opaque declarations of a variable-free
// obligation, which the soundness control rewrites back into True definitions.
var opaquePredicate = regexp.MustCompile(`(?m)^opaque (pred_\w+) : Prop$`)

// runLean elaborates src inside the lean-csp-prover checkout, which is what puts
// the library and Mathlib on the search path.
func runLean(t *testing.T, leanDir, name, src string) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), proverTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lake", "env", "lean", path)
	cmd.Dir = leanDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runIsabelle builds a session holding just this theory on top of the session its
// imports need. The file has to be named after the theory, and the session name
// has to be an identifier, so both are derived rather than taken verbatim.
func runIsabelle(t *testing.T, cspDir string, o obligation, src string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, o.theory+".thy"), []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	session := "Csdf_" + obligationir.Mangle(o.name)
	root := "session " + session + " = " + o.parent + " +\n  theories\n    " + o.theory + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ROOT"), []byte(root), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), proverTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, isabelleBin(), "build", "-d", cspDir, "-d", dir, session)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func isabelleBin() string {
	if bin := os.Getenv(envIsabelle); bin != "" {
		return bin
	}
	return "isabelle"
}

// leanDiagnostic matches a line Lean attributes to a source position. Only the
// intended sorry and the unused-variable linter are tolerated: an obligation is
// meant to elaborate with its theorem open and nothing else to say.
var (
	leanDiagnostic  = regexp.MustCompile(`^[^ ].*:\d+:\d+: (error|warning)`)
	toleratedLeanRe = regexp.MustCompile(`declaration uses .sorry.|unused variable`)
)

func unexpectedLeanDiagnostics(out string) []string {
	var res []string
	for _, line := range strings.Split(out, "\n") {
		if leanDiagnostic.MatchString(line) && !toleratedLeanRe.MatchString(line) {
			res = append(res, line)
		}
	}
	return res
}

// requireEnv returns the configured path, skipping the test when the prover is
// not available - unless CSDF_REQUIRE_PROVERS says a skip is a failure, which is
// what stops CI from passing without checking anything.
func requireEnv(t *testing.T, name string) string {
	t.Helper()
	if v := os.Getenv(name); v != "" {
		return v
	}
	if os.Getenv(envRequire) != "" {
		t.Fatalf("%s is set but %s is not, so this check would silently do nothing", envRequire, name)
	}
	t.Skipf("%s is not set; set it to a checkout of the prover library to run this check", name)
	return ""
}
