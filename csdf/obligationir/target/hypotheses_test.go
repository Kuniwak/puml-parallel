package target_test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
)

// theorem is the statement of the theorem name in out, from its name to its
// proof: the hypotheses it is stated under are in it.
func theorem(out, name string) string {
	m := regexp.MustCompile(`(?s)theorem ` + name + `\b.*?(oops|sorry)`).FindString(out)
	return m
}

// TestEachTheoremIsStatedUnderItsOwnHypotheses pins which hypotheses each
// theorem takes, of a spec that assumes fair and an impl that assumes
// steady. The refinement takes the axioms alone: an assumption of the impl
// could make it hold vacuously, and one of the spec, of liveness, is no
// claim traces or stable failures can state. initialisable_* takes the
// axioms alone too, since that a side can start must not rest on what it
// assumes of its runs; livelock_free_* takes its own side's assumptions,
// and the FD conclusion is conditional on them.
func TestEachTheoremIsStatedUnderItsOwnHypotheses(t *testing.T) {
	const (
		axiom  = "csdf_axiom_1"
		fair   = "assume_fair_S"
		steady = "assume_steady_I"
	)
	// The theorems of each model, and the hypotheses each is stated under;
	// every other hypothesis must be absent from it.
	theorems := map[obligationir.IRRefinementMode]map[string][]string{
		obligationir.IRRefinementModeTrace: {
			"refines_t": {axiom}, "initialisable_S": {axiom}, "initialisable_I": {axiom},
		},
		obligationir.IRRefinementModeStableFailure: {
			"refines_f": {axiom}, "initialisable_S": {axiom}, "initialisable_I": {axiom},
		},
		obligationir.IRRefinementModeFailuresDivergence: {
			"refines_f": {axiom}, "initialisable_S": {axiom}, "initialisable_I": {axiom},
			"livelock_free_S": {axiom, fair}, "livelock_free_I": {axiom, steady},
		},
	}
	type row struct {
		Mode       obligationir.IRRefinementMode
		BackEnd    target.Name
		Theorem    string
		Hypothesis string
		Present    bool
	}
	var rows []row
	for mode, ts := range theorems {
		for theorem, under := range ts {
			for _, h := range []string{axiom, fair, steady} {
				for _, backEnd := range []target.Name{target.NameIsabelle, target.NameLean} {
					rows = append(rows, row{mode, backEnd, theorem, h, slices.Contains(under, h)})
				}
			}
		}
	}

	for _, r := range rows {
		verb := "lacks"
		if r.Present {
			verb = "takes"
		}
		t.Run(fmt.Sprintf("in %s for %s, %s %s", r.Mode, r.BackEnd, r.Theorem, verb+" "+r.Hypothesis), func(t *testing.T) {
			// Setup: the spec assumes fair; the impl assumes steady.
			spec, impl := load(t, "map"), load(t, "fair_countdown")
			var sb strings.Builder

			// Execute
			err := target.CompileDiagrams(&sb, r.Mode, spec, impl, r.BackEnd)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			stmt := theorem(sb.String(), r.Theorem)
			if stmt == "" {
				t.Fatalf("no theorem %s in\n%s", r.Theorem, sb.String())
			}
			if got := strings.Contains(stmt, r.Hypothesis); got != r.Present {
				t.Errorf("%s under %s: %v, want %v: %s", r.Theorem, r.Hypothesis, got, r.Present, stmt)
			}
		})
	}
}

// TestTheLeanRefinementSilencesTheNamingLinter pins the option that keeps
// Mathlib's naming linter, which the "__" of a mangled name trips, from
// warning on every declaration.
func TestTheLeanRefinementSilencesTheNamingLinter(t *testing.T) {
	// Setup
	var sb strings.Builder

	// Execute
	err := target.CompileDiagrams(&sb, obligationir.IRRefinementModeTrace, load(t, "counter"), load(t, "countdown"), target.NameLean)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "\nset_option linter.style.nameCheck false\n") {
		t.Errorf("no set_option linter.style.nameCheck false in\n%s", sb.String())
	}
}
