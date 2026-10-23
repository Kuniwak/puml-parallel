package target_test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/google/go-cmp/cmp"
)

var (
	theoremName = regexp.MustCompile(`(?m)^theorem (\w+)`)
	// lemmaName is a lemma of the builtins, proved, as each back end spells it.
	lemmaName  = regexp.MustCompile(`(?m)^(?:lemma|theorem) (csdf_lemma_\d+)\b`)
	identifier = regexp.MustCompile(`\w+`)
)

// theorems maps each theorem of out to the hypotheses its statement names:
// the axioms and the assumptions, read as whole identifiers. A statement is
// ended by Isabelle's oops or Lean's :=; one ended by neither fails t, as
// the hypotheses read after it would be another's. The lemmas of the builtins
// and csdf_map_ext, are proved, not obligations, and are left out.
func theorems(t *testing.T, out string) map[string][]string {
	t.Helper()
	ts := map[string][]string{}
	locs := theoremName.FindAllStringSubmatchIndex(out, -1)
	for i, loc := range locs {
		end := len(out)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		stmt := out[loc[0]:end]
		name := out[loc[2]:loc[3]]
		if strings.HasPrefix(name, "csdf_") {
			continue // the model's own, proved
		}
		if k := strings.Index(stmt, "\n  oops"); k >= 0 {
			stmt = stmt[:k]
		} else if k := strings.Index(stmt, ":="); k >= 0 {
			stmt = stmt[:k]
		} else {
			t.Fatalf("the theorem %s ends neither by oops nor by := by: %s", name, stmt)
		}
		hs := []string{}
		for _, id := range identifier.FindAllString(stmt, -1) {
			if (strings.HasPrefix(id, "csdf_axiom_") || strings.HasPrefix(id, "assume_")) && !slices.Contains(hs, id) {
				hs = append(hs, id)
			}
		}
		slices.Sort(hs)
		ts[name] = hs
	}
	return ts
}

// TestEachTheoremIsStatedUnderItsOwnHypotheses pins the theorems of each
// model and the hypotheses each takes, of a spec that assumes fair and an
// impl that assumes steady. The refinement takes the axioms alone: an
// assumption of the impl could make it hold vacuously, and one of the spec,
// of liveness, is no claim traces or stable failures can state.
// initialisable_* takes the axioms alone too, since that a side can start
// must not rest on what it assumes of its runs; livelock_free_* takes its
// own side's assumptions, and the FD conclusion is conditional on them.
func TestEachTheoremIsStatedUnderItsOwnHypotheses(t *testing.T) {
	const fair, steady = "assume_fair_S", "assume_steady_I"
	// want are the theorems of a model and the assumptions each takes; no
	// theorem takes an axiom, the builtins being defined.
	want := map[obligationir.IRRefinementMode]map[string][]string{
		obligationir.IRRefinementModeTrace: {
			"refines_t": {}, "initialisable_S": {}, "initialisable_I": {},
		},
		obligationir.IRRefinementModeStableFailure: {
			"refines_f": {}, "initialisable_S": {}, "initialisable_I": {},
		},
		obligationir.IRRefinementModeFailuresDivergence: {
			"refines_f": {}, "initialisable_S": {}, "initialisable_I": {},
			"livelock_free_S": {fair}, "livelock_free_I": {steady},
		},
	}

	for mode, ts := range want {
		for _, backEnd := range []target.Name{target.NameIsabelle, target.NameLean} {
			t.Run(fmt.Sprintf("in %s for %s", mode, backEnd), func(t *testing.T) {
				// Setup: the spec assumes fair; the impl assumes steady.
				spec, impl := load(t, "map"), load(t, "fair_countdown")
				var sb strings.Builder

				// Execute
				err := target.CompileDiagrams(&sb, mode, spec, impl, backEnd)

				// Assert
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string][]string{}
				for name, own := range ts {
					expected[name] = append([]string{}, own...)
				}
				if diff := cmp.Diff(expected, theorems(t, sb.String())); diff != "" {
					t.Error(diff)
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

// TestTheBuiltinsAreDefinedAndTheirMeaningProved pins that an obligation
// reading the map builtins defines them and states what they mean as lemmas,
// with a proof, and as no axiom.
func TestTheBuiltinsAreDefinedAndTheirMeaningProved(t *testing.T) {
	for _, backEnd := range []target.Name{target.NameIsabelle, target.NameLean} {
		t.Run(string(backEnd), func(t *testing.T) {
			// Setup
			var sb strings.Builder

			// Execute
			err := target.CompileDiagram(&sb, load(t, "map"), backEnd)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			out := sb.String()
			if n := len(lemmaName.FindAllString(out, -1)); n != 2 {
				t.Errorf("%d lemmas, want the 2 of @empty and @frame", n)
			}
			for _, banned := range []string{"csdf_axiom", "axiom ", "sorry\n\ntheorem csdf_lemma", "consts csdf_in", "opaque csdf_in"} {
				if strings.Contains(out, banned) {
					t.Errorf("the obligation holds %q:\n%s", banned, out)
				}
			}
		})
	}
}
