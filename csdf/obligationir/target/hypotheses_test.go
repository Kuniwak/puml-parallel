package target_test

import (
	"fmt"
	"regexp"
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
	t_, f, fd := obligationir.IRRefinementModeTrace, obligationir.IRRefinementModeStableFailure, obligationir.IRRefinementModeFailuresDivergence
	rows := []struct {
		Mode       obligationir.IRRefinementMode
		Theorem    string
		Hypothesis string
		Present    bool
	}{
		{t_, "refines_t", "csdf_axiom_1", true},
		{t_, "refines_t", "assume_fair_S", false},
		{t_, "refines_t", "assume_steady_I", false},
		{t_, "initialisable_S", "csdf_axiom_1", true},
		{t_, "initialisable_S", "assume_fair_S", false},
		{t_, "initialisable_I", "assume_steady_I", false},
		{f, "refines_f", "csdf_axiom_1", true},
		{f, "refines_f", "assume_fair_S", false},
		{f, "refines_f", "assume_steady_I", false},
		{f, "initialisable_I", "assume_steady_I", false},
		{fd, "refines_f", "assume_steady_I", false},
		{fd, "refines_f", "assume_fair_S", false},
		{fd, "livelock_free_I", "assume_steady_I", true},
		{fd, "livelock_free_I", "assume_fair_S", false},
		{fd, "initialisable_I", "assume_steady_I", false},
	}

	for _, row := range rows {
		for _, backEnd := range []target.Name{target.NameIsabelle, target.NameLean} {
			t.Run(fmt.Sprintf("%s/%s/%s/%s/%v", row.Mode, backEnd, row.Theorem, row.Hypothesis, row.Present), func(t *testing.T) {
				// Setup
				spec, impl := load(t, "map"), load(t, "fair_countdown")
				var sb strings.Builder

				// Execute
				err := target.CompileDiagrams(&sb, row.Mode, spec, impl, backEnd)

				// Assert
				if err != nil {
					t.Fatal(err)
				}
				stmt := theorem(sb.String(), row.Theorem)
				if stmt == "" {
					t.Fatalf("no theorem %s in\n%s", row.Theorem, sb.String())
				}
				if got := strings.Contains(stmt, row.Hypothesis); got != row.Present {
					t.Errorf("%s under %s: %v, want %v: %s", row.Theorem, row.Hypothesis, got, row.Present, stmt)
				}
			})
		}
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
