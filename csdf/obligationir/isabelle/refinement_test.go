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
  where "SpecProc \<equiv> IF init_S THEN $(S_s0) ELSE DIV"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> IF init_I THEN $(I_t0) ELSE DIV"

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

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n = 10 *)
consts pred_xjezwh :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n > 0 *)
consts pred_1gdozh4 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n' < n *)
consts pred_1ini9wn :: "val \<Rightarrow> val \<Rightarrow> bool"

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
  "procfun (S_a v_n) =
     (IF (guard_S_L5 v_n \<and> (\<exists>v_n'. post_S_L5 v_n v_n'))
      THEN Alphabet Ev_dec -> (!<\<lambda>v_n'. Internal [v_n']> v_n':{v_n'. post_S_L5 v_n v_n'} .. $(S_a v_n'))
      ELSE STOP)"
| "procfun (I_a v_n) =
     (IF (guard_I_L5 v_n \<and> (\<exists>v_n'. post_I_L5 v_n v_n'))
      THEN Alphabet Ev_dec -> (!<\<lambda>v_n'. Internal [v_n']> v_n':{v_n'. post_I_L5 v_n v_n'} .. $(I_a v_n'))
      ELSE STOP)"

overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]

definition SpecProc :: "(PN, event) proc"
  where "SpecProc \<equiv> (!<\<lambda>v_n. Internal [v_n]> v_n:{v_n. init_S v_n} .. $(S_a v_n))"

definition ImplProc :: "(PN, event) proc"
  where "ImplProc \<equiv> (!<\<lambda>v_n. Internal [v_n]> v_n:{v_n. init_I v_n} .. $(I_a v_n))"

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
		`where "SpecProc \<equiv> (IF init_S THEN $(S_a) ELSE DIV) -- {HTau}"`,
		`where "ImplProc \<equiv> (IF init_I THEN $(I_a) ELSE DIV) -- {HTau}"`,
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
		`      THEN Alphabet HTau -> (!<\<lambda>v_n'. Internal [v_n']> v_n':{v_n'. post_S_L5 v_n v_n'} .. $(S_a v_n'))`,
		`.. $(S_a v_n)) -- {Alphabet HTau}"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestWriteRefinementEndEdgeIsSkip is stage (d): an end edge is CSP successful
// termination, offered alongside the state's other out-edges and guarded like
// them.
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
		"(* done *)\ndefinition guard_S_L5 :: \"bool\"\n  where \"guard_S_L5 \\<equiv> pred_",
		`      THEN Ev_x -> $(S_a)
      ELSE STOP)
     [+]
     (IF guard_S_L5
      THEN SKIP
      ELSE STOP)"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestWriteRefinementEndEdgeGuardTakesTheStateValuation(t *testing.T) {
	// The end guard constrains the source state's variables, so its alias is
	// applied to them.
	diagram := `@startuml
state "a" as a
a: n ; nat
[*] --> a
a --> [*] : n = 0
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeTrace, diagram, diagram)

	want := `  "procfun (S_a v_n) =
     (IF guard_S_L5 v_n
      THEN SKIP
      ELSE STOP)"`
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n%s", want, got)
	}
}

// TestWriteRefinementStableFailures is stage (e): the same encoding, imported
// into and stated in CSP-Prover's F model.
func TestWriteRefinementStableFailures(t *testing.T) {
	diagram := `@startuml
state "a" as a
[*] --> a
a --> a : x
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeStableFailure, diagram, diagram)

	for _, want := range []string{
		"theory Refinement_Obligation\n  imports CSP_F.CSP_F\nbegin\n",
		`theorem refines_f: "SpecProc <=F ImplProc"
  oops`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "<=T") {
		t.Errorf("want no trace refinement in stable-failures mode\n%s", got)
	}
}

// TestWriteRefinementFailuresDivergence is stage (f). CSP-Prover has no FD model,
// so fd reduces to <=F plus a divergence-freedom obligation per side: the F model
// cannot observe divergence at all, and for divergence-free processes the two
// refinements coincide. The divergence-freedom obligation is the csdflivelockfree
// obligation inlined per side, over side-qualified relations.
func TestWriteRefinementFailuresDivergence(t *testing.T) {
	diagram := `@startuml
state "a" as a
[*] --> a
a --> a : tau
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeFailuresDivergence, diagram, diagram)

	for _, want := range []string{
		"theory Refinement_Obligation\n  imports CSP_F.CSP_F\nbegin\n",
		// The state datatype's constructors carry St_ because the process-name
		// datatype in the same theory already claims S_a and I_a.
		"datatype st_S = St_S_a\n",
		"datatype st_I = St_I_a\n",
		`definition step_S :: "st_S \<Rightarrow> st_S \<Rightarrow> bool"`,
		`inductive reachable_S :: "st_S \<Rightarrow> bool"`,
		`definition tau_step_I :: "st_I \<Rightarrow> st_I \<Rightarrow> bool"`,
		`theorem livelock_free_S: "wf_on {s. reachable_S s} {(s', s). tau_step_S s s'}"
  oops`,
		`theorem livelock_free_I: "wf_on {s. reachable_I s} {(s', s). tau_step_I s s'}"
  oops`,
		`theorem refines_f: "SpecProc <=F ImplProc"
  oops`,
		// The reduction has to be stated, or the theory looks like it proves the
		// wrong thing.
		"for\n   divergence-free processes, FD refinement coincides with <=F",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

func TestWriteRefinementStableFailuresOmitsTheReductionNote(t *testing.T) {
	// The reduction is what fd means; in f mode there is nothing to reduce.
	diagram := `@startuml
state "a" as a
[*] --> a
a --> a : tau
@enduml
`
	got := compileRefinement(t, obligationir.IRRefinementModeStableFailure, diagram, diagram)
	if strings.Contains(got, "FD refinement") {
		t.Errorf("want no reduction note in stable-failures mode\n%s", got)
	}
	if strings.Contains(got, "wf_on") {
		t.Errorf("want no divergence-freedom obligation in stable-failures mode\n%s", got)
	}
}

func TestWriteRefinementFailuresDivergenceSkipsStructurallyFreeSides(t *testing.T) {
	// A side with no reachable tau-cycle is divergence free whatever the
	// predicates say, so it gets a note instead of an obligation - the same rule
	// csdflivelockfree follows.
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
	if !strings.Contains(got, "(* Spec is livelock free structurally: no reachable tau-cycle. *)") {
		t.Errorf("want the structural note for the spec\n%s", got)
	}
	if !strings.Contains(got, `theorem livelock_free_I: "wf_on`) {
		t.Errorf("want the obligation for the diverging impl\n%s", got)
	}
	// Its relations are pointless without an obligation to state over them.
	if strings.Contains(got, "step_S") {
		t.Errorf("want no transition system for a structurally livelock-free spec\n%s", got)
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
		`  "procfun (S_a v_n v_m) =`,
		`(\<exists>v_n' v_m'. post_S_L6 v_n v_m v_n' v_m')`,
		`(!<\<lambda>(v_n', v_m'). Internal [v_n', v_m']> (v_n', v_m'):{(v_n', v_m'). post_S_L6 v_n v_m v_n' v_m'} .. $(S_a v_n' v_m'))`,
		`where "SpecProc \<equiv> (!<\<lambda>(v_n, v_m). Internal [v_n, v_m]> (v_n, v_m):{(v_n, v_m). init_S v_n v_m} .. $(S_a v_n v_m))"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n%s", want, got)
		}
	}
}

// TestWriteRefinementManglesNonIdentifierNames mirrors the lean backend: names
// CSDF allows but Isabelle does not have to reach the theory as identifiers,
// under the same spellings, with a table recording the originals.
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
		"Ev_choose_u28_a_u20_product_u29_",
		"S_vm_u2d_idle v_1st v_end",
		`   choose_u28_a_u20_product_u29_ = "choose(a product)"`,
		`   vm_u2d_idle = "vm-idle"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRefinement() does not contain %q; got:\n%s", want, got)
		}
	}
}

// TestWriteRefinementLeavesNaturalLanguagePredicatesUninterpreted mirrors the
// lean backend: a predicate nobody has formalised is an uninterpreted constant,
// not a True definition, which would be a different diagram rather than a
// placeholder.
func TestWriteRefinementLeavesNaturalLanguagePredicatesUninterpreted(t *testing.T) {
	got := compileRefinement(t, obligationir.IRRefinementModeStableFailure, `@startuml
state "s0" as s0
s0: n
[*] --> s0
s0 --> s0 : a ; n is positive ; n' is n
@enduml
`, `@startuml
state "t0" as t0
t0: n
[*] --> t0
t0 --> t0 : a
@enduml
`)

	for _, want := range []string{
		"(* TODO(csdf): not formalised; this predicate is uninterpreted. *)",
		"(* n is positive *)",
		"consts pred_",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRefinement() does not contain %q; got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "is positive *)\ndefinition pred_") {
		t.Errorf("WriteRefinement() still defines a natural-language predicate; got:\n%s", got)
	}
	if !strings.Contains(got, "(* true *)\ndefinition pred_") {
		t.Errorf("WriteRefinement() does not define the literal true predicate; got:\n%s", got)
	}
}

// TestWriteRefinementUnsatisfiableInitIsDIV mirrors the lean backend: a diagram
// that cannot start denotes DIV whether or not it has state variables. The
// replicated internal choice over the empty set is DIV in the F model
// (cspF_Rep_int_choice_DIV), and a variable-free diagram's valuation domain is a
// one-point set, so an unsatisfiable init leaves the empty set there too.
func TestWriteRefinementUnsatisfiableInitIsDIV(t *testing.T) {
	got := compileRefinement(t, obligationir.IRRefinementModeStableFailure, `@startuml
state "s0" as s0
[*] --> s0 : nothing satisfies this
s0 --> s0 : a
@enduml
`, `@startuml
state "t0" as t0
[*] --> t0
t0 --> t0 : a
@enduml
`)

	want := `IF init_S THEN $(S_s0) ELSE DIV`
	if !strings.Contains(got, want) {
		t.Errorf("WriteRefinement() does not contain %q; got:\n%s", want, got)
	}
}

// TestWriteRefinementFailuresDivergenceRequiresAnInitialValuation mirrors the
// lean backend's fd obligation: DIV diverges on the empty trace, so each side's
// initial valuation set has to be non-empty for the reduction of FD refinement to
// <=F to hold.
func TestWriteRefinementFailuresDivergenceRequiresAnInitialValuation(t *testing.T) {
	got := compileRefinement(t, obligationir.IRRefinementModeFailuresDivergence, `@startuml
state "s0" as s0
s0: n
[*] --> s0 : n' is 0
s0 --> s0 : a
@enduml
`, `@startuml
state "t0" as t0
[*] --> t0
t0 --> t0 : a
@enduml
`)

	for _, want := range []string{
		"theorem initialisable_S: \"\\<exists>v_n. init_S v_n\"\n  oops",
		"theorem initialisable_I: \"init_I\"\n  oops",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WriteRefinement() does not contain %q; got:\n%s", want, got)
		}
	}
}
