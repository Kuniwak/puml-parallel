package target_test

import (
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
// theorem takes. The refinement takes the axioms alone: an assumption of
// the impl could make it hold vacuously, and one of the spec, of liveness,
// is no claim traces or stable failures can state. Each side's assumptions
// are hypotheses of that side's lemmas only, and the initialisability of a
// side is a lemma in every model, since a side that cannot start makes a
// refinement vacuous in T and F too.
func TestEachTheoremIsStatedUnderItsOwnHypotheses(t *testing.T) {
	type want struct {
		Theorem    string
		Has, Lacks []string
	}
	modes := map[obligationir.IRRefinementMode][]want{
		obligationir.IRRefinementModeTrace: {
			{Theorem: "refines_t", Has: []string{"csdf_axiom_1"}, Lacks: []string{"assume_fair_S"}},
			{Theorem: "initialisable_S", Has: []string{"csdf_axiom_1", "assume_fair_S"}},
			{Theorem: "initialisable_I", Lacks: []string{"assume_fair_S"}},
		},
		obligationir.IRRefinementModeStableFailure: {
			{Theorem: "refines_f", Has: []string{"csdf_axiom_1"}, Lacks: []string{"assume_fair_S"}},
			{Theorem: "initialisable_S", Has: []string{"assume_fair_S"}},
		},
		obligationir.IRRefinementModeFailuresDivergence: {
			{Theorem: "refines_f", Has: []string{"csdf_axiom_1"}, Lacks: []string{"assume_fair_S"}},
			{Theorem: "livelock_free_S", Has: []string{"csdf_axiom_1", "assume_fair_S"}},
			{Theorem: "livelock_free_I", Lacks: []string{"assume_fair_S"}},
			{Theorem: "initialisable_S", Has: []string{"assume_fair_S"}},
		},
	}

	for mode, wants := range modes {
		for _, backEnd := range []target.Name{target.NameIsabelle, target.NameLean} {
			t.Run(string(mode)+"/"+string(backEnd), func(t *testing.T) {
				// Setup: the spec assumes fair; the impl assumes nothing.
				var sb strings.Builder

				// Execute
				err := target.CompileDiagrams(&sb, mode, load(t, "map"), load(t, "countdown"), backEnd)

				// Assert
				if err != nil {
					t.Fatal(err)
				}
				for _, w := range wants {
					stmt := theorem(sb.String(), w.Theorem)
					if stmt == "" {
						t.Errorf("no theorem %s in\n%s", w.Theorem, sb.String())
						continue
					}
					for _, h := range w.Has {
						if !strings.Contains(stmt, h) {
							t.Errorf("%s is not stated under %s: %s", w.Theorem, h, stmt)
						}
					}
					for _, h := range w.Lacks {
						if strings.Contains(stmt, h) {
							t.Errorf("%s is stated under %s: %s", w.Theorem, h, stmt)
						}
					}
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
