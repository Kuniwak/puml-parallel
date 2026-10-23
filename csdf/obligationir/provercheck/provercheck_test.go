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
// CSDF_REQUIRE_PROVERS names the provers whose absence is a failure rather than a
// skip, as a comma-separated list of "lean", "isabelle" and "all". CI sets it to
// whichever prover the job installs, so a job that lost its library fails instead
// of reporting a pass having checked nothing.
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

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
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

// The provers, named as CSDF_REQUIRE_PROVERS names them. A job that installs one
// prover cannot be asked to have the other, so the requirement is per prover
// rather than one flag for the package.
const (
	proverLean     = "lean"
	proverIsabelle = "isabelle"
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
	ir, err := obligationir.BuildLivelockFree(load(t, path))
	if err != nil {
		t.Fatalf("BuildLivelockFree(%s) error = %v", name, err)
	}

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
	ir, err := obligationir.BuildRefinement(mode, load(t, specPath), load(t, implPath))
	if err != nil {
		t.Fatalf("BuildRefinement(%s) error = %v", name, err)
	}

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

func load(t *testing.T, path string) *core.Diagram {
	t.Helper()
	d, err := coreio.Load(path)
	if err != nil {
		t.Fatalf("coreio.Load(%q) error = %v", path, err)
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
		countdown   = "../testdata/formula/countdown.puml"
		counter     = "../testdata/formula/counter.puml"
		maps        = "../testdata/formula/map.puml"
	)
	return []obligation{
		livelockObligation(t, "livelock_countdown", countdown),
		livelockObligation(t, "livelock_maps_and_lemmas", maps),
		livelockObligation(t, "livelock_every_builtin", "../testdata/formula/builtins.puml"),
		livelockObligation(t, "livelock_nested_maps", "../testdata/formula/nested.puml"),
		livelockObligation(t, "livelock_hostile_names", hostileSpec),
		refinementObligation(t, "refinement_ground_f", obligationir.IRRefinementModeStableFailure, groundSpec, groundImpl),
		refinementObligation(t, "refinement_values_t", obligationir.IRRefinementModeTrace, counter, countdown),
		refinementObligation(t, "refinement_values_f", obligationir.IRRefinementModeStableFailure, counter, counter),
		refinementObligation(t, "refinement_lemmas_fd", obligationir.IRRefinementModeFailuresDivergence, maps, countdown),
		refinementObligation(t, "refinement_hostile_t", obligationir.IRRefinementModeTrace, hostileSpec, hostileImpl),
		refinementObligation(t, "refinement_hostile_f", obligationir.IRRefinementModeStableFailure, hostileSpec, hostileImpl),
		refinementObligation(t, "refinement_hostile_fd", obligationir.IRRefinementModeFailuresDivergence, hostileSpec, hostileImpl),
		// A diagram compared with itself refines itself, so this is the plainest
		// completeness case there is: it must at least be checkable.
		refinementObligation(t, "refinement_reflexive_f", obligationir.IRRefinementModeStableFailure, hostileSpec, hostileSpec),
	}
}

func TestLeanAcceptsTheGeneratedObligations(t *testing.T) {
	leanDir := requireEnv(t, proverLean, envLean)
	for _, o := range obligations(t) {
		t.Run(o.name, func(t *testing.T) {
			src := o.leanSrc + printAxioms(o.leanSrc)
			out, err := runLean(t, leanDir, o.name+".lean", src)
			if err != nil {
				t.Fatalf("lean rejected the obligation: %v\n%s\n--- source ---\n%s", err, out, src)
			}
			if diags := unexpectedLeanDiagnostics(out); len(diags) > 0 {
				t.Errorf("lean reported %v\n--- source ---\n%s", diags, src)
			}
			if bad := foreignAxioms(out); len(bad) > 0 {
				t.Errorf("the model rests on %v\n%s", bad, out)
			}
		})
	}
}

func TestIsabelleAcceptsTheGeneratedObligations(t *testing.T) {
	cspDir := requireEnv(t, proverIsabelle, envCSP)
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
	leanDir := requireEnv(t, proverLean, envLean)
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
	cspDir := requireEnv(t, proverIsabelle, envCSP)
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
	leanDir := requireEnv(t, proverLean, envLean)
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
	cspDir := requireEnv(t, proverIsabelle, envCSP)
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
	leanDir := requireEnv(t, proverLean, envLean)
	o := refinementObligation(t, "disabled", obligationir.IRRefinementModeStableFailure,
		"testdata/disabled_spec.puml", "testdata/disabled_impl.puml")

	// The atom names come from the IR rather than from the fixture's text: a
	// name that does not exist would be auto-bound as an implicit argument and
	// the probe would fail for that reason instead, which is exactly what the
	// True control below caught the first time round. autoImplicit is switched
	// off so that cannot happen silently again.
	ir, err := obligationir.BuildRefinement(obligationir.IRRefinementModeStableFailure,
		load(t, "testdata/disabled_spec.puml"), load(t, "testdata/disabled_impl.puml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ir.Impl.Edges) != 1 {
		t.Fatalf("the fixture has %d Impl edges, want exactly the one guarded edge", len(ir.Impl.Edges))
	}
	syms, err := obligationir.Symbols(nil, ir.Impl.Edges[0].Step)
	if err != nil {
		t.Fatal(err)
	}
	var atoms []string
	for _, s := range syms {
		if s.Kind == obligationir.KindAtom {
			atoms = append(atoms, s.Name)
		}
	}
	probe := "\nset_option autoImplicit false in\nexample : " + strings.Join(atoms, " ∧ ") + " := by trivial\n"

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

// opaquePredicate matches the nullary atoms of a variable-free obligation,
// which the soundness control rewrites into True definitions.
var opaquePredicate = regexp.MustCompile(`(?m)^opaque (atom_\w+) : Prop$`)

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
	root := isabelle.Root(session, o.parent, o.theory)
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

// requireEnv returns the path configured for prover, skipping the test when it is
// not available - unless CSDF_REQUIRE_PROVERS names that prover, which is what
// stops CI from passing without checking anything.
func requireEnv(t *testing.T, prover, name string) string {
	t.Helper()
	if v := os.Getenv(name); v != "" {
		return v
	}
	if isRequired(prover) {
		t.Fatalf("%s requires %s but %s is not set, so this check would silently do nothing", envRequire, prover, name)
	}
	t.Skipf("%s is not set; set it to a checkout of the prover library to run this check", name)
	return ""
}

// isRequired reports whether CSDF_REQUIRE_PROVERS names prover, either directly
// or through "all".
func isRequired(prover string) bool {
	for _, want := range strings.Split(os.Getenv(envRequire), ",") {
		switch strings.TrimSpace(want) {
		case prover, "all":
			return true
		}
	}
	return false
}

// TestTheProversDoNotDischargeAFalseRefinement is a control on the encoding:
// an Impl that does go, with no clause that could stop it, refines no Spec
// that never does, so no proof of the trace refinement may go through. It is
// weak - a tactic can fail for reasons of its own - but a generator whose
// refinement held vacuously, say under a hypothesis that is false, would let
// the tactic through.
func TestTheProversDoNotDischargeAFalseRefinement(t *testing.T) {
	o := refinementObligation(t, "false_refinement_t", obligationir.IRRefinementModeTrace,
		"testdata/never_spec.puml", "testdata/go_impl.puml")
	t.Run("lean", func(t *testing.T) {
		leanDir := requireEnv(t, proverLean, envLean)
		src := strings.Replace(o.leanSrc, ": refTfix SpecProc ImplProc := by\n  sorry", ": refTfix SpecProc ImplProc := by\n  trivial", 1)
		if src == o.leanSrc {
			t.Fatal("the obligation no longer states refines_t as this control expects")
		}
		if out, err := runLean(t, leanDir, "false_refinement.lean", src); err == nil {
			t.Errorf("lean discharged a false refinement\n%s", out)
		}
	})
	t.Run("isabelle", func(t *testing.T) {
		cspDir := requireEnv(t, proverIsabelle, envCSP)
		src := strings.Replace(o.isaSrc, "\"SpecProc <=T ImplProc\"\n  oops", "\"SpecProc <=T ImplProc\"\n  by auto", 1)
		if src == o.isaSrc {
			t.Fatal("the obligation no longer states refines_t as this control expects")
		}
		if out, err := runIsabelle(t, cspDir, o, src); err == nil {
			t.Errorf("isabelle discharged a false refinement\n%s", out)
		}
	})
}

// TestTheBuiltinsAreAModel holds the definitions of the builtins to the
// meaning a refinement needs of them, by proofs a prover checks: a frame of
// no key leaves a well-formed map as it was, so an edge whose post is
// @frame(m', m) ∧ m' ≠ m is disabled; and a diagram starting at a state of
// its local diagram with an empty map, its types formalised, can start.
func TestTheBuiltinsAreAModel(t *testing.T) {
	// Setup
	ir, err := obligationir.BuildRefinement(obligationir.IRRefinementModeTrace,
		load(t, "../testdata/formula/builtins.puml"), load(t, "../testdata/formula/countdown.puml"))
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{"type_t": "True", "type_m": "True"}
	leanEdits := lean.Edits{
		Types: types,
		Proofs: map[string]string{"initialisable_S": strings.Join([]string{
			"refine ⟨.ValRecord (.ValInt 0) [], .ValMap [], trivial, ?_, trivial, ?_, rfl, trivial, ?_⟩",
			"  · exact csdf_wf.record _ _ (csdf_wf.int 0) (by simp)",
			"  · exact csdf_wf.map [] List.nodup_nil List.Pairwise.nil.sortedLE (by simp) (by simp)",
			"  · intro k ⟨v, hv⟩; simp at hv"}, "\n")},
		Lemmas: []string{"theorem frame0_keeps (n m : Val) (wn : csdf_wf n) (wm : csdf_wf m) (h : csdf_frame0 n m) : n = m :=\n" +
			"  csdf_map_ext n m wn wm (And.left h) (And.left (And.right h)) (fun k =>\n" +
			"    ⟨⟨(And.right (And.right h) k trivial).1, fun hm => ((And.right (And.right h) k trivial).2 hm).1⟩,\n" +
			"     fun hn => ((And.right (And.right h) k trivial).2 ((And.right (And.right h) k trivial).1 hn)).2⟩)"},
	}
	isaEdits := isabelle.Edits{
		Types: types,
		Proofs: map[string]string{"initialisable_S": "apply (rule exI[of _ \"ValRecord (ValInt 0) []\"], rule exI[of _ \"ValMap []\"])\n" +
			"  apply (auto simp: type_t_def type_m_def csdf_at_def state__u2euA_def csdf_empty_def csdf_is_map_def csdf_in_def intro: csdf_wf.intros)\n  done"},
		Lemmas: []string{"lemma frame0_keeps: \"csdf_wf n \\<Longrightarrow> csdf_wf m \\<Longrightarrow> csdf_frame0 n m \\<Longrightarrow> n = m\"\n" +
			"  by (rule csdf_map_ext) (auto simp: csdf_frame0_def)"},
	}
	var leanSrc, isaSrc strings.Builder
	if err := lean.WriteRefinementEdited(&leanSrc, ir, leanEdits); err != nil {
		t.Fatal(err)
	}
	if err := isabelle.WriteRefinementEdited(&isaSrc, ir, isaEdits); err != nil {
		t.Fatal(err)
	}
	o := obligation{name: "builtins_model", theory: "Refinement_Obligation", parent: "CSP_T"}

	t.Run("lean", func(t *testing.T) {
		leanDir := requireEnv(t, proverLean, envLean)

		// Execute
		src := leanSrc.String() + "\n#print axioms Refinement_Obligation.frame0_keeps\n#print axioms Refinement_Obligation.initialisable_S\n"
		out, err := runLean(t, leanDir, "builtins_model.lean", src)

		// Assert
		if err != nil {
			t.Fatalf("lean rejected the model: %v\n%s\n--- source ---\n%s", err, out, src)
		}
		if diags := unexpectedLeanDiagnostics(out); len(diags) > 0 {
			t.Errorf("lean reported %v", diags)
		}
		if bad := foreignAxioms(out); len(bad) > 0 {
			t.Errorf("the model rests on %v\n%s", bad, out)
		}
	})
	t.Run("isabelle", func(t *testing.T) {
		cspDir := requireEnv(t, proverIsabelle, envCSP)

		// Execute
		out, err := runIsabelle(t, cspDir, o, isaSrc.String())

		// Assert
		if err != nil {
			t.Fatalf("isabelle rejected the model: %v\n%s\n--- source ---\n%s", err, out, isaSrc.String())
		}
	})
}

// modelTheorem is a theorem of the model a generated file proves.
var modelTheorem = regexp.MustCompile(`(?m)^theorem (csdf_\w+)`)

// printAxioms asks Lean what each theorem of the model in src rests on.
func printAxioms(src string) string {
	prefix := ""
	if strings.Contains(src, "namespace Refinement_Obligation") {
		prefix = "Refinement_Obligation."
	}
	var sb strings.Builder
	for _, m := range modelTheorem.FindAllStringSubmatch(src, -1) {
		sb.WriteString("\n#print axioms " + prefix + m[1])
	}
	return sb.String() + "\n"
}

// TestNoObligationDeclaresAnAxiom holds every generated file to defining
// what it assumes of the builtins rather than declaring it: an axiom that
// contradicted another would prove anything.
func TestNoObligationDeclaresAnAxiom(t *testing.T) {
	for _, o := range obligations(t) {
		t.Run(o.name, func(t *testing.T) {
			if regexp.MustCompile(`(?m)^axiomatization|^axiom `).MatchString(o.isaSrc + o.leanSrc) {
				t.Errorf("%s declares an axiom", o.name)
			}
		})
	}
}

// axiomsLine is what #print axioms says of a declaration.
var axiomsLine = regexp.MustCompile(`depends on axioms: \[([^\]]*)\]`)

// foreignAxioms are the axioms out says a declaration rests on beyond Lean's
// own: propext, Quot.sound and Classical.choice.
func foreignAxioms(out string) []string {
	var bad []string
	for _, m := range axiomsLine.FindAllStringSubmatch(out, -1) {
		for _, a := range strings.Split(m[1], ",") {
			switch a = strings.TrimSpace(a); a {
			case "propext", "Quot.sound", "Classical.choice":
			default:
				bad = append(bad, a)
			}
		}
	}
	return bad
}
