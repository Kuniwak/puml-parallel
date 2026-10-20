package lean

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

const assumeDiagram = `@startuml
state "a" as a
a: n ; Nat
[*] --> a : n = 10
a --> a : tau ; n > 0 ; n' = n - 1
note as fair
  assume n reaches 0 eventually
end note
@enduml
`

// An assumption is a proposition nobody has formalised, declared like a
// predicate, and the theorem is stated under it.
func TestCompileStatesTheLivelockTheoremUnderTheAssumptions(t *testing.T) {
	got := MustCompileLivelockFreeString(assumeDiagram)

	for _, want := range []string{
		"-- TODO(csdf): not formalised; this assumption is uninterpreted.\n-- n reaches 0 eventually\nopaque assume_fair : Prop",
		"theorem livelock_free (h_assume_fair : assume_fair) :\n    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by\n  sorry",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestCompileStatesTheRefinementTheoremUnderEachSidesAssumptions(t *testing.T) {
	spec := "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : x\nnote as spec_fair\n  assume x keeps coming\nend note\n@enduml\n"
	impl := "@startuml\nstate \"a\" as a\n[*] --> a\na --> a : x\nnote as fair\n  assume x is eventually taken\nend note\n@enduml\n"
	got := compileRefinement(t, obligationir.IRRefinementModeStableFailure, spec, impl)

	for _, want := range []string{
		"opaque assume_spec__fair_S : Prop",
		"opaque assume_fair_I : Prop",
		"theorem refines_f (h_assume_spec__fair_S : assume_spec__fair_S) (h_assume_fair_I : assume_fair_I) : refFfix SpecProc ImplProc := by\n  sorry",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestCompileStatesAPerSideLivelockTheoremUnderThatSidesAssumptions(t *testing.T) {
	free := "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : x\n@enduml\n"
	got := compileRefinement(t, obligationir.IRRefinementModeFailuresDivergence, free, assumeDiagram)

	want := "theorem livelock_free_I (h_assume_fair_I : assume_fair_I) :\n    WellFounded (fun s' s => Reachable_I s ∧ tauStep_I s s') := by\n  sorry"
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n%s", want, got)
	}
}
