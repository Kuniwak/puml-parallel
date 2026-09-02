package lean

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func compileRefinement(t *testing.T, mode obligationir.IRRefinementMode, spec, impl string) string {
	t.Helper()
	ir := obligationir.BuildRefinement(mode, csdf.MustParse(spec), csdf.MustParse(impl))
	var b bytes.Buffer
	if err := WriteRefinement(&b, ir); err != nil {
		t.Fatalf("WriteRefinement() error = %v", err)
	}
	return b.String()
}

// TestWriteRefinementGroundTrace is stage (a): no state variables, no tau, no end
// edge. It mirrors the isabelle backend line for line - same predicate ids, same
// side-qualified alias names - so the two skeletons can be read side by side.
func TestWriteRefinementGroundTrace(t *testing.T) {
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, `@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : a
@enduml
`, `@startuml
state "t0" as t0
[*] --> t0
t0 --> t0 : a
t0 --> t0 : b
@enduml
`)

	want := `import LeanCspProver.CSP_T.CSP_T_Main

-- The guards below are propositions, so the Prop-valued procIte is used rather
-- than the Bool-valued proc.IF.
attribute [local instance] Classical.propDecidable

noncomputable section

namespace Refinement_Obligation

inductive event where
  | Ev_a
  | Ev_b
deriving Inhabited

-- true
def pred_1ygzo25 : Prop := True

-- true
def init_S : Prop := pred_1ygzo25

-- true
def guard_S_L4 : Prop := pred_1ygzo25

-- true
def post_S_L4 : Prop := pred_1ygzo25

-- true
def init_I : Prop := pred_1ygzo25

-- true
def guard_I_L4 : Prop := pred_1ygzo25

-- true
def post_I_L4 : Prop := pred_1ygzo25

-- true
def guard_I_L5 : Prop := pred_1ygzo25

-- true
def post_I_L5 : Prop := pred_1ygzo25

inductive PN where
  | S_s0
  | I_t0
deriving Inhabited

def procfun : PN → proc PN event
  | PN.S_s0 =>
    procIte (guard_S_L4 ∧ post_S_L4)
      (event.Ev_a ~> proc.Proc_name PN.S_s0)
      proc.STOP
  | PN.I_t0 =>
    procIte (guard_I_L4 ∧ post_I_L4)
      (event.Ev_a ~> proc.Proc_name PN.I_t0)
      proc.STOP
    [+]
    procIte (guard_I_L5 ∧ post_I_L5)
      (event.Ev_b ~> proc.Proc_name PN.I_t0)
      proc.STOP

instance Set_procfun : HasPNfun PN event where
  PNfun := procfun

instance Set_FPmode : HasFPmode where
  FPmode := fpmode.CMSmode

def SpecProc : proc PN event :=
  procIte init_S (proc.Proc_name PN.S_s0) proc.STOP

def ImplProc : proc PN event :=
  procIte init_I (proc.Proc_name PN.I_t0) proc.STOP

-- Every trace of the Impl diagram is a trace of the Spec diagram: refT P1 M1 M2 P2
-- unfolds to semTf P2 M2 <= semTf P1 M1.
theorem refines_t : refTfix SpecProc ImplProc := by
  sorry

end Refinement_Obligation

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

// TestWriteRefinementWithVars is stage (b): the post predicate generally admits
// several successor valuations, which the diagram picks internally, so the
// successor is a replicated internal choice. lean-csp-prover can only index one
// by the event type, hence the layered event datatype and the Internal
// injection - the same shape the isabelle backend emits.
func TestWriteRefinementWithVars(t *testing.T) {
	diagram := `@startuml
state "a" as a
a: n ; nat
[*] --> a : n = 10
a --> a : dec ; n > 0 ; n' < n
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	for _, want := range []string{
		"inductive Val where\n  | ValInt (i : Int)",
		"inductive alphabet where\n  | Ev_dec\nderiving Inhabited\n",
		"inductive event where\n  | Alphabet (a : alphabet)\n  | Internal (vs : List Val)\nderiving Inhabited\n",
		// The same predicate ids and alias names as the isabelle skeleton.
		"def pred_1ini9wn (n n' : Val) : Prop := True",
		"def post_S_L5 : Val → Val → Prop := pred_1ini9wn",
		// The enabledness conjunct: without it an edge whose post is unsatisfiable
		// would still offer its event.
		"procIte (guard_S_L5 n ∧ ∃ n', post_S_L5 n n')",
		"(event.Alphabet alphabet.Ev_dec ~> Rep_int_choice_f (fun n' => event.Internal [n']) {n' | post_S_L5 n n'}",
		"(fun n' => proc.Proc_name (PN.S_a n')))",
		"Rep_int_choice_f (fun n => event.Internal [n]) {n | init_S n}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestWriteRefinementTuplesMultipleVars(t *testing.T) {
	// A state carrying several variables binds a tuple, injected as a list.
	diagram := `@startuml
state "a" as a
a: n ; nat
a: m ; nat
[*] --> a
a --> a : step ; true ; n' = m
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	for _, want := range []string{
		"  | PN.S_a n m =>",
		"∃ n' m', post_S_L6 n m n' m'",
		"Rep_int_choice_f (fun (n', m') => event.Internal [n', m']) {(n', m') | post_S_L6 n m n' m'}",
		"(fun (n', m') => proc.Proc_name (PN.S_a n' m'))",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestWriteRefinementHidesTau is stage (c): a tau edge becomes a prefix on the
// reserved event HTau, hidden once at the outermost level.
func TestWriteRefinementHidesTau(t *testing.T) {
	diagram := `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> b : x
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	for _, want := range []string{
		// HTau goes last: it is not part of the visible alphabet.
		"inductive event where\n  | Ev_x\n  | HTau\nderiving Inhabited",
		"(event.HTau ~> proc.Proc_name PN.S_b)",
		`def SpecProc : proc PN event :=
  proc.Hiding
    (procIte init_S (proc.Proc_name PN.S_a) proc.STOP)
    {event.HTau}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestWriteRefinementEndEdgeIsSkip is stage (d).
func TestWriteRefinementEndEdgeIsSkip(t *testing.T) {
	diagram := `@startuml
state "a" as a
[*] --> a
a --> a : x
a --> [*] : done
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	for _, want := range []string{
		"-- done\ndef guard_S_L5 : Prop := pred_",
		`    [+]
    procIte (guard_S_L5)
      proc.SKIP
      proc.STOP`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestWriteRefinementStableFailures is stage (e).
func TestWriteRefinementStableFailures(t *testing.T) {
	diagram := `@startuml
state "a" as a
[*] --> a
a --> a : x
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeStableFailure, diagram, diagram)

	for _, want := range []string{
		"import LeanCspProver.CSP_F.CSP_F_Main",
		"theorem refines_f : refFfix SpecProc ImplProc := by\n  sorry",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "refTfix") {
		t.Errorf("want no trace refinement in stable-failures mode\n%s", got)
	}
	if strings.Contains(got, "WellFounded") {
		t.Errorf("want no divergence-freedom obligation in stable-failures mode\n%s", got)
	}
}

// TestWriteRefinementFailuresDivergence is stage (f): the reduction to the F
// refinement plus a divergence-freedom obligation per side.
func TestWriteRefinementFailuresDivergence(t *testing.T) {
	diagram := `@startuml
state "a" as a
[*] --> a
a --> a : tau
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeFailuresDivergence, diagram, diagram)

	for _, want := range []string{
		"import LeanCspProver.CSP_F.CSP_F_Main",
		// The state datatype is distinct from the process-name datatype.
		"inductive St_S where\n  | St_S_a",
		"def step_S (s s' : St_S) : Prop :=",
		"inductive Reachable_S : St_S → Prop where",
		"def tauStep_I (s s' : St_I) : Prop :=",
		"theorem livelock_free_S :\n    WellFounded (fun s' s => Reachable_S s ∧ tauStep_S s s') := by\n  sorry",
		"theorem livelock_free_I :\n    WellFounded (fun s' s => Reachable_I s ∧ tauStep_I s s') := by\n  sorry",
		"for\n-- divergence-free processes, FD refinement coincides with the F refinement.",
		"theorem refines_f : refFfix SpecProc ImplProc := by",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestWriteRefinementFailuresDivergenceSkipsStructurallyFreeSides(t *testing.T) {
	free := `@startuml
state "a" as a
[*] --> a
a --> a : x
@enduml
`
	diverging := `@startuml
state "a" as a
[*] --> a
a --> a : tau
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeFailuresDivergence, free, diverging)

	if strings.Contains(got, "livelock_free_S") {
		t.Errorf("want no obligation for a structurally livelock-free spec\n%s", got)
	}
	if !strings.Contains(got, "-- Spec is livelock free structurally: no reachable tau-cycle.") {
		t.Errorf("want the structural note for the spec\n%s", got)
	}
	if !strings.Contains(got, "theorem livelock_free_I :") {
		t.Errorf("want the obligation for the diverging impl\n%s", got)
	}
	if strings.Contains(got, "step_S") {
		t.Errorf("want no transition system for a structurally livelock-free spec\n%s", got)
	}
}

// TestWriteRefinementPredicateSharingKeepsOccurrenceArgs pins that an edge
// applies its guard and post to the variables its own source and target states
// bind, not to whichever occurrence happened to be recorded last under the shared
// predicate id. Two edges whose predicates share a text and an argument signature
// but bind differently named variables must still produce closed terms.
func TestWriteRefinementPredicateSharingKeepsOccurrenceArgs(t *testing.T) {
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, `@startuml
state "s0" as s0
s0: x
state "s1" as s1
s1: y
[*] --> s0 : holds
s0 --> s1 : a ; holds ; holds
s1 --> s0 : b ; holds ; holds
@enduml
`, `@startuml
state "t0" as t0
t0: z
[*] --> t0 : holds
t0 --> t0 : a ; holds ; holds
@enduml
`)

	for _, want := range []string{
		"guard_S_L7 x ∧ ∃ y', post_S_L7 x y'",
		"guard_S_L8 y ∧ ∃ x', post_S_L8 y x'",
		"guard_I_L5 z ∧ ∃ z', post_I_L5 z z'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRefinement() does not contain %q; got:\n%s", want, got)
		}
	}
}

// TestWriteRefinementManglesNonIdentifierNames pins that names CSDF allows but
// Lean does not - an event with parentheses and spaces, a hyphenated state id, a
// state variable that starts with a digit or spells a keyword - reach the theory
// as identifiers, with a table recording the originals. Emitting them verbatim
// produced a file Lean cannot parse, so even a diagram compared with itself had
// no checkable obligation.
func TestWriteRefinementManglesNonIdentifierNames(t *testing.T) {
	diagram := `@startuml
state "vm-idle" as vm-idle
vm-idle: 1st
vm-idle: end
[*] --> vm-idle
vm-idle --> vm-idle : choose(a product)
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	for _, want := range []string{
		"| Ev_choose_u28_a_u20_product_u29_",
		"| S_vm_u2d_idle (u_1st : Val) (end_ : Val)",
		"PN.S_vm_u2d_idle u_1st end_ =>",
		`--   choose_u28_a_u20_product_u29_ = "choose(a product)"`,
		`--   end_ = "end"`,
		`--   u_1st = "1st"`,
		`--   vm_u2d_idle = "vm-idle"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRefinement() does not contain %q; got:\n%s", want, got)
		}
	}
}
