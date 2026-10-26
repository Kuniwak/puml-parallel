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
		livelockObligation(t, "livelock_literals", "../testdata/formula/literals.puml"),
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
// @frame(m', m) ∧ m' ≠ m is disabled; a map is represented by exactly one
// well-formed list, so the entries of distinct keys say which map it is; an
// update by a key, and a frame of a key dropping it, has exactly one
// well-formed successor, so a post of either neither disables an edge nor
// lets it branch, while a bare frame of a key is deliberately
// nondeterministic on it, having two; and a diagram starting at a state of
// its local diagram with an empty map, its types formalised, can start.
func TestTheBuiltinsAreAModel(t *testing.T) {
	// Setup
	ir, err := obligationir.BuildRefinement(obligationir.IRRefinementModeTrace,
		load(t, "../testdata/formula/builtins.puml"), load(t, "../testdata/formula/countdown.puml"))
	if err != nil {
		t.Fatal(err)
	}
	// The lemmas the model is held to, each proved in both provers.
	lemmas := []struct{ desc, name, lean, isa string }{
		{"a frame of no key keeps a map", "frame0_keeps",
			"theorem frame0_keeps (n m : Val) (wn : csdf_wf n) (wm : csdf_wf m) (h : csdf_frame0 n m) : n = m :=\n" +
				"  csdf_map_ext n m wn wm (And.left h) (And.left (And.right h)) (fun k =>\n" +
				"    ⟨⟨(And.right (And.right h) k trivial).1, fun hm => ((And.right (And.right h) k trivial).2 hm).1⟩,\n" +
				"     fun hn => ((And.right (And.right h) k trivial).2 ((And.right (And.right h) k trivial).1 hn)).2⟩)",
			"lemma frame0_keeps: \"csdf_wf n \\<Longrightarrow> csdf_wf m \\<Longrightarrow> csdf_frame0 n m \\<Longrightarrow> n = m\"\n" +
				"  by (rule csdf_map_ext) (auto simp: csdf_frame0_def)"},
		{"a map is represented by exactly one well-formed list", "canonical_unique",
			`theorem canonical_unique (kvs : List (Val × Val)) (n : (kvs.map Prod.fst).Nodup)
    (w1 : ∀ p ∈ kvs, csdf_wf p.1) (w2 : ∀ p ∈ kvs, csdf_wf p.2) :
    ∃! l : List (Val × Val), csdf_wf (.ValMap l) ∧ ∀ p, p ∈ l ↔ p ∈ kvs := by
  let le : Val × Val → Val × Val → Bool := fun p q => decide (p.1 ≤ q.1)
  have perm : (kvs.mergeSort le).Perm kvs := List.mergeSort_perm kvs le
  have mem : ∀ p, p ∈ kvs.mergeSort le ↔ p ∈ kvs := fun p => perm.mem_iff
  have so : (kvs.mergeSort le).Pairwise (fun p q => le p q) :=
    List.pairwise_mergeSort (fun a b c h1 h2 => by simp [le] at *; exact le_trans h1 h2)
      (fun a b => by simp [le]; exact le_total _ _) kvs
  have wf : csdf_wf (.ValMap (kvs.mergeSort le)) :=
    csdf_wf.map _ ((perm.map Prod.fst).nodup_iff.mpr n)
      (List.sortedLE_iff_pairwise.mpr (List.pairwise_map.mpr (so.imp (by simp [le]))))
      (fun p hp => w1 p ((mem p).mp hp)) (fun p hp => w2 p ((mem p).mp hp))
  refine ⟨kvs.mergeSort le, ⟨wf, mem⟩, ?_⟩
  rintro l ⟨wl, hl⟩
  have e := csdf_map_ext (.ValMap l) (.ValMap (kvs.mergeSort le)) wl wf trivial trivial
    (fun k => by simp [csdf_in, csdf_index, hl, mem])
  injection e`,
			`lemma canonical_unique:
  assumes "distinct (map fst kvs)" "\<forall>p\<in>set kvs. csdf_wf (fst p)" "\<forall>p\<in>set kvs. csdf_wf (snd p)"
  shows "\<exists>!l. csdf_wf (ValMap l) \<and> set l = set kvs"
proof -
  have d: "distinct (map fst (sort_key fst kvs))"
    using assms(1) by (metis distinct_card card_distinct length_map length_sort set_map set_sort)
  have w: "csdf_wf (ValMap (sort_key fst kvs))"
    using d assms(2,3) by (auto intro!: csdf_wf.intros simp: sorted_sort_key)
  show ?thesis
  proof (rule ex1I[of _ "sort_key fst kvs"])
    show "csdf_wf (ValMap (sort_key fst kvs)) \<and> set (sort_key fst kvs) = set kvs" using w by simp
  next
    fix l assume l: "csdf_wf (ValMap l) \<and> set l = set kvs"
    have dl: "distinct (map fst l)" using l by (auto elim: csdf_wf_map)
    have "map_of l = map_of (sort_key fst kvs)" using map_of_inject_set[OF dl d] l by simp
    then have "ValMap l = ValMap (sort_key fst kvs)"
      using l by (intro csdf_map_ext[OF conjunct1[OF l] w]) (auto simp: csdf_is_map_def csdf_in_def csdf_index_def)
    then show "l = sort_key fst kvs" by simp
  qed
qed`},
		{"an update has a well-formed successor", "update_exists",
			`-- The entries of a map but those of a key.
open Classical in
noncomputable abbrev without (k : Val) (ms : List (Val × Val)) : List (Val × Val) :=
  ms.filter (fun p => decide (p.1 ≠ k))

theorem mem_without {k : Val} {ms : List (Val × Val)} {p : Val × Val} :
    p ∈ without k ms ↔ p ∈ ms ∧ p.1 ≠ k := by
  simp [without]

theorem index_of (l : List (Val × Val)) (n : (l.map Prod.fst).Nodup) (k v : Val) (h : (k, v) ∈ l) :
    csdf_index (.ValMap l) k = v := by
  have e : ∃ v, (k, v) ∈ l := ⟨v, h⟩
  simp only [csdf_index, dif_pos e]
  exact csdf_unique l n k _ _ (Classical.choose_spec e) h

theorem update_exists (m k v : Val) (wm : csdf_wf m) (im : csdf_is_map m) (wk : csdf_wf k) (wv : csdf_wf v) :
    ∃ n, csdf_wf n ∧ csdf_update n m k v := by
  cases wm with
  | map ms nd so w1 w2 => ?_
  | _ => simp [csdf_is_map] at im
  classical
  let r : Val × Val → Val × Val → Prop := fun p q => p.1 ≤ q.1
  haveI : Std.Total r := ⟨fun a b => le_total a.1 b.1⟩
  haveI : IsTrans (Val × Val) r := ⟨fun _ _ _ => le_trans⟩
  let l := (without k ms).orderedInsert r (k, v)
  have lt : (without k ms).Pairwise (fun p q => p.1 < q.1) := (csdf_sorted_entries ms nd so).filter _
  have le : l.Pairwise r := (lt.imp le_of_lt).orderedInsert _ _
  have st0 := List.sortedLT_iff_nodup_and_sortedLE.mp (List.sortedLT_iff_pairwise.mpr (List.pairwise_map.mpr lt))
  have nd' : (l.map Prod.fst).Nodup := by
    refine ((List.perm_orderedInsert r (k, v) _).map Prod.fst).nodup_iff.mpr ?_
    refine List.nodup_cons.mpr ⟨?_, st0.1⟩
    simp only [List.mem_map]
    rintro ⟨p, hp, e⟩; exact (mem_without.mp hp).2 e
  have mem : ∀ p, p ∈ l ↔ p = (k, v) ∨ p ∈ without k ms := fun p => List.mem_orderedInsert r
  refine ⟨.ValMap l, csdf_wf.map _ nd' (List.sortedLE_iff_pairwise.mpr (List.pairwise_map.mpr le)) ?_ ?_,
    trivial, trivial, ⟨v, (mem _).mpr (.inl rfl)⟩, index_of _ nd' k v ((mem _).mpr (.inl rfl)), fun j e => ⟨?_, ?_⟩⟩
  · intro p hp; rcases (mem p).mp hp with rfl | hp; exacts [wk, w1 p (mem_without.mp hp).1]
  · intro p hp; rcases (mem p).mp hp with rfl | hp; exacts [wv, w2 p (mem_without.mp hp).1]
  · rintro ⟨w, hw⟩
    rcases (mem _).mp hw with h | h
    · exact absurd (Prod.mk.inj h).1 e
    · exact ⟨w, (mem_without.mp h).1⟩
  · rintro ⟨w, hw⟩
    have hn : (j, w) ∈ l := (mem _).mpr (.inr (mem_without.mpr ⟨hw, e⟩))
    exact ⟨⟨w, hn⟩, (index_of _ nd' j w hn).trans (index_of ms nd j w hw).symm⟩`,
			`lemma csdf_index_of: "distinct (map fst l) \<Longrightarrow> (j, w) \<in> set l \<Longrightarrow> csdf_index (ValMap l) j = w"
  by (simp add: csdf_index_def)

lemma update_exists:
  assumes "csdf_wf m" "csdf_is_map m" "csdf_wf k" "csdf_wf v"
  shows "\<exists>n. csdf_wf n \<and> csdf_update n m k v"
proof -
  obtain ms where m: "m = ValMap ms" using assms(2) by (cases m) (auto simp: csdf_is_map_def)
  have w: "distinct (map fst ms)" "sorted (map fst ms)" "\<forall>p\<in>set ms. csdf_wf (fst p)" "\<forall>p\<in>set ms. csdf_wf (snd p)"
    using assms(1) unfolding m by (auto elim: csdf_wf_map)
  let ?r = "filter (\<lambda>p. fst p \<noteq> k) ms"
  let ?l = "insort_key fst (k, v) ?r"
  have dr: "distinct (map fst ?r)" using w(1) by (simp add: distinct_map_filter)
  have mi: "\<And>xs. map fst (insort_key fst (k, v) xs) = insort k (map fst xs)" by (induct_tac xs) auto
  have ml: "map fst ?l = insort k (map fst ?r)" by (rule mi)
  have d: "distinct (map fst ?l)" unfolding ml using dr by (auto simp: distinct_insort)
  have "csdf_wf (ValMap ?l)"
    by (rule csdf_wf.intros) (use w d assms(3,4) in \<open>auto simp: ml sorted_insort sorted_filter set_insort_key\<close>)
  moreover have "csdf_update (ValMap ?l) m k v"
    using w(1) d by (auto simp: m csdf_update_def csdf_is_map_def csdf_in_def csdf_index_of set_insort_key)
  ultimately show ?thesis by blast
qed`},
		{"an update has at most one well-formed successor", "update_unique",
			`theorem update_unique (m k v : Val) : ∀ n₁ n₂, csdf_wf n₁ → csdf_wf n₂ →
    csdf_update n₁ m k v → csdf_update n₂ m k v → n₁ = n₂ := by
  intro n₁ n₂ w₁ w₂ ⟨i₁, _, hk₁, hv₁, hj₁⟩ ⟨i₂, _, hk₂, hv₂, hj₂⟩
  refine csdf_map_ext n₁ n₂ w₁ w₂ i₁ i₂ fun j => ?_
  by_cases e : j = k
  · subst e; exact ⟨⟨fun _ => hk₂, fun _ => hk₁⟩, fun _ => hv₁.trans hv₂.symm⟩
  · have a := hj₁ j e; have b := hj₂ j e
    exact ⟨⟨fun h => (b.2 (a.1 h)).1, fun h => (a.2 (b.1 h)).1⟩,
      fun h => (a.2 (a.1 h)).2.trans (b.2 (a.1 h)).2.symm⟩`,
			`lemma update_unique: "csdf_wf n1 \<Longrightarrow> csdf_wf n2 \<Longrightarrow> csdf_update n1 m k v \<Longrightarrow> csdf_update n2 m k v \<Longrightarrow> n1 = n2"
  apply (rule csdf_map_ext, assumption+)
    apply (auto simp: csdf_update_def)
  by metis+`},
		{"a frame of a key, dropping it, has a well-formed successor", "frame1_exists",
			`theorem frame1_exists (m k : Val) (wm : csdf_wf m) (im : csdf_is_map m) :
    ∃ n, csdf_wf n ∧ csdf_frame1 n m k ∧ ¬csdf_in k n := by
  cases wm with
  | map ms nd so w1 w2 => ?_
  | _ => simp [csdf_is_map] at im
  have lt : (without k ms).Pairwise (fun p q => p.1 < q.1) := (csdf_sorted_entries ms nd so).filter _
  have st := List.sortedLT_iff_nodup_and_sortedLE.mp (List.sortedLT_iff_pairwise.mpr (List.pairwise_map.mpr lt))
  refine ⟨.ValMap (without k ms), csdf_wf.map _ st.1 st.2
    (fun p hp => w1 p (mem_without.mp hp).1) (fun p hp => w2 p (mem_without.mp hp).1),
    ⟨trivial, trivial, fun j e => ⟨fun ⟨v, hv⟩ => ⟨v, (mem_without.mp hv).1⟩, fun ⟨v, hv⟩ => ?_⟩⟩, ?_⟩
  · have hn : (j, v) ∈ without k ms := mem_without.mpr ⟨hv, e⟩
    exact ⟨⟨v, hn⟩, (index_of _ st.1 j v hn).trans (index_of ms nd j v hv).symm⟩
  · rintro ⟨v, hv⟩; exact (mem_without.mp hv).2 rfl`,
			`lemma frame1_exists:
  assumes "csdf_wf m" "csdf_is_map m"
  shows "\<exists>n. csdf_wf n \<and> csdf_frame1 n m k \<and> \<not> csdf_in k n"
proof -
  obtain ms where m: "m = ValMap ms" using assms(2) by (cases m) (auto simp: csdf_is_map_def)
  have w: "distinct (map fst ms)" "sorted (map fst ms)" "\<forall>p\<in>set ms. csdf_wf (fst p)" "\<forall>p\<in>set ms. csdf_wf (snd p)"
    using assms(1) unfolding m by (auto elim: csdf_wf_map)
  let ?l = "filter (\<lambda>p. fst p \<noteq> k) ms"
  have d: "distinct (map fst ?l)" using w(1) by (simp add: distinct_map_filter)
  have "csdf_wf (ValMap ?l)"
    by (rule csdf_wf.intros) (use w d in \<open>auto simp: sorted_filter\<close>)
  moreover have "csdf_frame1 (ValMap ?l) m k \<and> \<not> csdf_in k (ValMap ?l)"
    using w(1) d by (auto simp: m csdf_frame1_def csdf_is_map_def csdf_in_def csdf_index_of)
  ultimately show ?thesis by blast
qed`},
		// A bare @frame(m', m, k) says nothing of k in m', so it is
		// deliberately nondeterministic on k: m' may hold k at any value or
		// not at all. A post pins k down with @frame and @notin, or @update.
		{"a bare frame of a key has two different well-formed successors", "frame1_branches",
			`theorem frame1_branches (m k v : Val) (wm : csdf_wf m) (im : csdf_is_map m) (wk : csdf_wf k) (wv : csdf_wf v) :
    ∃ n₁ n₂, csdf_wf n₁ ∧ csdf_wf n₂ ∧ csdf_frame1 n₁ m k ∧ csdf_frame1 n₂ m k ∧ n₁ ≠ n₂ := by
  obtain ⟨n₁, w₁, i₁, i₁', hk₁, _, hj₁⟩ := update_exists m k v wm im wk wv
  obtain ⟨n₂, w₂, f₂, hk₂⟩ := frame1_exists m k wm im
  exact ⟨n₁, n₂, w₁, w₂, ⟨i₁, i₁', hj₁⟩, f₂, fun e => hk₂ (e ▸ hk₁)⟩`,
			`lemma frame1_branches:
  assumes "csdf_wf m" "csdf_is_map m" "csdf_wf k" "csdf_wf v"
  shows "\<exists>n1 n2. csdf_wf n1 \<and> csdf_wf n2 \<and> csdf_frame1 n1 m k \<and> csdf_frame1 n2 m k \<and> n1 \<noteq> n2"
proof -
  obtain n1 where n1: "csdf_wf n1" "csdf_update n1 m k v" using update_exists[OF assms] by blast
  obtain n2 where n2: "csdf_wf n2" "csdf_frame1 n2 m k" "\<not> csdf_in k n2" using frame1_exists[OF assms(1,2)] by blast
  have "csdf_frame1 n1 m k" using n1(2) by (simp add: csdf_update_def csdf_frame1_def)
  moreover have "n1 \<noteq> n2" using n1(2) n2(3) by (auto simp: csdf_update_def)
  ultimately show ?thesis using n1(1) n2(1,2) by blast
qed`},
		{"a frame of a key, dropping it, has at most one well-formed successor", "frame1_unique",
			`theorem frame1_unique (m k : Val) : ∀ n₁ n₂, csdf_wf n₁ → csdf_wf n₂ →
    (csdf_frame1 n₁ m k ∧ ¬csdf_in k n₁) → (csdf_frame1 n₂ m k ∧ ¬csdf_in k n₂) → n₁ = n₂ := by
  intro n₁ n₂ w₁ w₂ ⟨⟨i₁, _, hj₁⟩, hk₁⟩ ⟨⟨i₂, _, hj₂⟩, hk₂⟩
  refine csdf_map_ext n₁ n₂ w₁ w₂ i₁ i₂ fun j => ?_
  by_cases e : j = k
  · subst e; exact ⟨⟨fun h => absurd h hk₁, fun h => absurd h hk₂⟩, fun h => absurd h hk₁⟩
  · have a := hj₁ j e; have b := hj₂ j e
    exact ⟨⟨fun h => (b.2 (a.1 h)).1, fun h => (a.2 (b.1 h)).1⟩,
      fun h => (a.2 (a.1 h)).2.trans (b.2 (a.1 h)).2.symm⟩`,
			`lemma frame1_unique: "csdf_wf n1 \<Longrightarrow> csdf_wf n2 \<Longrightarrow> csdf_frame1 n1 m k \<and> \<not> csdf_in k n1 \<Longrightarrow> csdf_frame1 n2 m k \<and> \<not> csdf_in k n2 \<Longrightarrow> n1 = n2"
  apply (rule csdf_map_ext, assumption+)
    apply (auto simp: csdf_frame1_def)
  by metis+`},
	}
	var leanLemmas, isaLemmas []string
	var printAxioms strings.Builder
	for _, l := range lemmas {
		leanLemmas = append(leanLemmas, l.lean)
		isaLemmas = append(isaLemmas, l.isa)
		printAxioms.WriteString("#print axioms Refinement_Obligation." + l.name + "\n")
	}
	types := map[string]string{"type_t": "True", "type_m": "True", "type__u9375u": "True"}
	leanEdits := lean.Edits{
		Types: types,
		Proofs: map[string]string{"initialisable_S": strings.Join([]string{
			"refine ⟨.ValRecord (.ValInt 0) [], .ValMap [], trivial, ?_, trivial, ?_, rfl, trivial, ?_⟩",
			"  · exact csdf_wf.record _ _ (csdf_wf.int 0) (by simp)",
			"  · exact csdf_wf.map [] List.nodup_nil List.Pairwise.nil.sortedLE (by simp) (by simp)",
			"  · intro k ⟨v, hv⟩; simp at hv"}, "\n")},
		Lemmas: leanLemmas,
	}
	isaEdits := isabelle.Edits{
		Types: types,
		Proofs: map[string]string{"initialisable_S": "apply (rule exI[of _ \"ValRecord (ValInt 0) []\"], rule exI[of _ \"ValMap []\"])\n" +
			"  apply (auto simp: type_t_def type_m_def csdf_at_def state__u2euA_def csdf_empty_def csdf_is_map_def csdf_in_def intro: csdf_wf.intros)\n  done"},
		Lemmas: isaLemmas,
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
		src := leanSrc.String() + "\n" + printAxioms.String() + "#print axioms Refinement_Obligation.initialisable_S\n"
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
