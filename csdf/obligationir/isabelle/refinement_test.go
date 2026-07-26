package isabelle

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

// TestWriteRefinementWithVars is stage (b): state variables and opaque
// predicates. The post predicate generally admits several successor valuations,
// which the diagram picks internally, so the successor is a replicated internal
// choice; CSP-Prover can only index one by the event type, hence the layered
// event datatype and the Internal injection. Note the enabledness conjunct
// (\<exists>n'. post ...) in the guard: without it an edge whose post is
// unsatisfiable would still offer its event.
func TestWriteRefinementWithVars(t *testing.T) {
	diagram := `@startuml
state "a" as a
a: n ; nat
[*] --> a : n = 10
a --> a : dec ; n > 0 ; n' < n
@enduml
`
	// The same text on both sides: the state ids and line numbers collide, so
	// every declaration is side-qualified, while the predicates - which do not
	// depend on a location - dedupe into one placeholder layer.
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	want := `theory Refinement_Obligation
  imports CSP_T.CSP_T
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"

datatype alphabet = Ev_dec

(* Internal only indexes the replicated internal choices below, via
   Rep_int_choice_f; no process performs it, so it changes no trace and no
   refusal. The injections into it are of the form \<lambda>(x, y). Internal [x, y],
   whose inj side condition is discharged by (simp add: inj_def). *)
datatype event = Alphabet alphabet
  | Internal "val list"

(* n = 10 *)
definition pred_xjezwh :: "val \<Rightarrow> bool"
  where "pred_xjezwh n \<equiv> True"

(* n > 0 *)
definition pred_1gdozh4 :: "val \<Rightarrow> bool"
  where "pred_1gdozh4 n \<equiv> True"

(* n' < n *)
definition pred_1ini9wn :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "pred_1ini9wn n n' \<equiv> True"

(* n = 10 *)
definition init_S :: "val \<Rightarrow> bool"
  where "init_S \<equiv> pred_xjezwh"

(* n > 0 *)
definition guard_S_L5 :: "val \<Rightarrow> bool"
  where "guard_S_L5 \<equiv> pred_1gdozh4"

(* n' < n *)
definition post_S_L5 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_S_L5 \<equiv> pred_1ini9wn"

(* n = 10 *)
definition init_I :: "val \<Rightarrow> bool"
  where "init_I \<equiv> pred_xjezwh"

(* n > 0 *)
definition guard_I_L5 :: "val \<Rightarrow> bool"
  where "guard_I_L5 \<equiv> pred_1gdozh4"

(* n' < n *)
definition post_I_L5 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_I_L5 \<equiv> pred_1ini9wn"

datatype PN = S_a val
  | I_a val

primrec
  procfun :: "(PN, event) pnfun"
where
  "procfun (S_a n) =
     (IF (guard_S_L5 n \<and> (\<exists>n'. post_S_L5 n n'))
      THEN Alphabet Ev_dec -> (!<\<lambda>n'. Internal [n']> n':{n'. post_S_L5 n n'} .. $(S_a n'))
      ELSE STOP)"
| "procfun (I_a n) =
     (IF (guard_I_L5 n \<and> (\<exists>n'. post_I_L5 n n'))
      THEN Alphabet Ev_dec -> (!<\<lambda>n'. Internal [n']> n':{n'. post_I_L5 n n'} .. $(I_a n'))
      ELSE STOP)"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> (!<\<lambda>n. Internal [n]> n:{n. init_S n} .. $(S_a n))"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> (!<\<lambda>n. Internal [n]> n:{n. init_I n} .. $(I_a n))"

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

// TestWriteRefinementHidesTau is stage (c): a tau edge becomes a prefix on the
// reserved event HTau, hidden once at the outermost level. Hiding is what makes
// it internal - it produces instability, and on an infinite run of tau steps,
// divergence - which a plain visible event would not.
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
		"datatype event = Ev_x\n  | HTau\n",
		"      THEN HTau -> $(S_b)\n",
		`where "SpecProc \<equiv> (IF init_S THEN $(S_a) ELSE STOP) -- {HTau}"`,
		`where "ImplProc \<equiv> (IF init_I THEN $(I_a) ELSE STOP) -- {HTau}"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestWriteRefinementHidesTauUnderTheEventLayer(t *testing.T) {
	// With valuations in play the event type is layered, so the hidden set names
	// the wrapped event.
	diagram := `@startuml
state "a" as a
a: n ; nat
[*] --> a
a --> a : tau ; n > 0 ; n' < n
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	for _, want := range []string{
		"datatype alphabet = HTau\n",
		`      THEN Alphabet HTau -> (!<\<lambda>n'. Internal [n']> n':{n'. post_S_L5 n n'} .. $(S_a n'))`,
		`.. $(S_a n)) -- {Alphabet HTau}"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestWriteRefinementTuplesMultipleVars pins the binder a state carrying several
// variables gets: the internal choice ranges over tuples, injected into the event
// type as a list.
func TestWriteRefinementTuplesMultipleVars(t *testing.T) {
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
		`  "procfun (S_a n m) =`,
		`(\<exists>n' m'. post_S_L6 n m n' m')`,
		`(!<\<lambda>(n', m'). Internal [n', m']> (n', m'):{(n', m'). post_S_L6 n m n' m'} .. $(S_a n' m'))`,
		`where "SpecProc \<equiv> (!<\<lambda>(n, m). Internal [n, m]> (n, m):{(n, m). init_S n m} .. $(S_a n m))"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}
