package isabelle

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

func TestCompileStatesTheLivelockTheoremUnderTheAssumptions(t *testing.T) {
	got := MustCompileLivelockFreeString(assumeDiagram)

	for _, want := range []string{
		"(* TODO(csdf): not formalised; this assumption is uninterpreted. *)\n(* n reaches 0 eventually *)\nconsts assume_fair :: \"bool\"",
		"theorem livelock_free:\n  assumes \"assume_fair\"\n  shows \"wf_on {s. reachable s} {(s', s). tau_step s s'}\"\n  oops",
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
		"consts assume_spec__fair_S :: \"bool\"",
		"consts assume_fair_I :: \"bool\"",
		"theorem refines_f:\n  assumes \"assume_spec__fair_S\" and \"assume_fair_I\"\n  shows \"SpecProc <=F ImplProc\"\n  oops",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestCompileStatesAPerSideLivelockTheoremUnderThatSidesAssumptions(t *testing.T) {
	free := "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : x\n@enduml\n"
	got := compileRefinement(t, obligationir.IRRefinementModeFailuresDivergence, free, assumeDiagram)

	want := "theorem livelock_free_I:\n  assumes \"assume_fair_I\"\n  shows \"wf_on {s. reachable_I s} {(s', s). tau_step_I s s'}\"\n  oops"
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n%s", want, got)
	}
}
