package isabelle

import (
	"bytes"
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
// edge. Each state becomes a process name whose body is the external choice over
// its out-edges, each guarded by its enabledness, and the obligation is the
// one-line trace refinement. Both sides share one predicate layer, so the single
// True placeholder pred_1ygzo25 backs every alias on both sides.
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

	want := `theory Refinement_Obligation
  imports CSP_T.CSP_T
begin

datatype event = Ev_a
  | Ev_b

(* true *)
definition pred_1ygzo25 :: "bool"
  where "pred_1ygzo25 \<equiv> True"

(* true *)
definition init_S :: "bool"
  where "init_S \<equiv> pred_1ygzo25"

(* true *)
definition guard_S_L4 :: "bool"
  where "guard_S_L4 \<equiv> pred_1ygzo25"

(* true *)
definition post_S_L4 :: "bool"
  where "post_S_L4 \<equiv> pred_1ygzo25"

(* true *)
definition init_I :: "bool"
  where "init_I \<equiv> pred_1ygzo25"

(* true *)
definition guard_I_L4 :: "bool"
  where "guard_I_L4 \<equiv> pred_1ygzo25"

(* true *)
definition post_I_L4 :: "bool"
  where "post_I_L4 \<equiv> pred_1ygzo25"

(* true *)
definition guard_I_L5 :: "bool"
  where "guard_I_L5 \<equiv> pred_1ygzo25"

(* true *)
definition post_I_L5 :: "bool"
  where "post_I_L5 \<equiv> pred_1ygzo25"

datatype PN = S_s0
  | I_t0

primrec
  procfun :: "(PN, event) pnfun"
where
  "procfun (S_s0) =
     (IF (guard_S_L4 \<and> post_S_L4)
      THEN Ev_a -> $(S_s0)
      ELSE STOP)"
| "procfun (I_t0) =
     (IF (guard_I_L4 \<and> post_I_L4)
      THEN Ev_a -> $(I_t0)
      ELSE STOP)
     [+]
     (IF (guard_I_L5 \<and> post_I_L5)
      THEN Ev_b -> $(I_t0)
      ELSE STOP)"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> IF init_S THEN $(S_s0) ELSE STOP"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> IF init_I THEN $(I_t0) ELSE STOP"

(* Every trace of the Impl diagram is a trace of the Spec diagram: in CSP-Prover
   P <=T Q unfolds to traces Q <= traces P. *)
theorem refines_t: "SpecProc <=T ImplProc"
  oops

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}
