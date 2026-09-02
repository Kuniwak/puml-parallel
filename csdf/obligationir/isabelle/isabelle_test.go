package isabelle

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCompileTauSelfLoopWithVars(t *testing.T) {
	// A guarded tau self-loop carrying a variable becomes: the datatype, the
	// guard/post as True-placeholder definitions (each preceded by its
	// natural-language text), the init predicate, the step and tau_step relations,
	// the inductive reachable predicate, and the livelock_free theorem left as
	// oops. The theorem is restricted to reachable states: over all of st it would
	// be strictly stronger than livelock freedom and often false.
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
a: n ; nat
[*] --> a : n = 10
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `theory Livelock_Obligation
  imports Main
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"

datatype st = a val (* type: (n :: nat) *)

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n = 10 *)
consts pred_xjezwh :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n > 0 *)
consts pred_1gdozh4 :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n' = n - 1 *)
consts pred_1nuhmrf :: "val \<Rightarrow> val \<Rightarrow> bool"

(* n = 10 *)
definition init :: "val \<Rightarrow> bool"
  where "init \<equiv> pred_xjezwh"

(* n > 0 *)
definition guard_L5 :: "val \<Rightarrow> bool"
  where "guard_L5 \<equiv> pred_1gdozh4"

(* n' = n - 1 *)
definition post_L5 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_L5 \<equiv> pred_1nuhmrf"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init n \<Longrightarrow> reachable (a n)"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

theorem livelock_free: "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileStructurallyFreeEmitsNoObligation(t *testing.T) {
	// A visible-only chain has no tau edge, so tau_step is False, no predicate
	// definitions are emitted, and — being structurally livelock free — no oops
	// obligation is emitted, only a note.
	got := MustCompileLivelockFreeString(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	want := `theory Livelock_Obligation
  imports Main
begin

(* Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation. *)

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileUntypedVariableIsval(t *testing.T) {
	// An untyped state variable is still a val value; no declared-type comment is
	// emitted because nothing was declared.
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
a: n
[*] --> a : n = 10
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `theory Livelock_Obligation
  imports Main
begin

datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"

datatype st = a val (* type: (n :: any) *)

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n' = n - 1 *)
consts pred_7ydc3w :: "val \<Rightarrow> val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n = 10 *)
consts pred_icpx2l :: "val \<Rightarrow> bool"

(* TODO(csdf): not formalised; this predicate is uninterpreted. *)
(* n > 0 *)
consts pred_1e81hjg :: "val \<Rightarrow> bool"

(* n = 10 *)
definition init :: "val \<Rightarrow> bool"
  where "init \<equiv> pred_icpx2l"

(* n > 0 *)
definition guard_L5 :: "val \<Rightarrow> bool"
  where "guard_L5 \<equiv> pred_1e81hjg"

(* n' = n - 1 *)
definition post_L5 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_L5 \<equiv> pred_7ydc3w"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init n \<Longrightarrow> reachable (a n)"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

theorem livelock_free: "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileMultipleTauEdgesAreParenthesisedDisjuncts(t *testing.T) {
	// Two tau edges become a parenthesised disjunction inside the where-clause so
	// neither existential captures the other's clause. Variable-free states make
	// every predicate the same omitted "true", so init and both edges share one
	// placeholder, and the base rule takes no binder.
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)

	want := `theory Livelock_Obligation
  imports Main
begin

datatype st = a
  | b

(* true *)
definition pred_1ygzo25 :: "bool"
  where "pred_1ygzo25 \<equiv> True"

(* true *)
definition init :: "bool"
  where "init \<equiv> pred_1ygzo25"

(* true *)
definition guard_L5 :: "bool"
  where "guard_L5 \<equiv> pred_1ygzo25"

(* true *)
definition post_L5 :: "bool"
  where "post_L5 \<equiv> pred_1ygzo25"

(* true *)
definition guard_L6 :: "bool"
  where "guard_L6 \<equiv> pred_1ygzo25"

(* true *)
definition post_L6 :: "bool"
  where "post_L6 \<equiv> pred_1ygzo25"

definition step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "step s s' \<equiv> (s = a \<and> s' = b \<and> guard_L5 \<and> post_L5)
    \<or> (s = b \<and> s' = a \<and> guard_L6 \<and> post_L6)"

inductive reachable :: "st \<Rightarrow> bool"
  where base: "init \<Longrightarrow> reachable a"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (s = a \<and> s' = b \<and> guard_L5 \<and> post_L5)
    \<or> (s = b \<and> s' = a \<and> guard_L6 \<and> post_L6)"

theorem livelock_free: "wf_on {s. reachable s} {(s', s). tau_step s s'}"
  oops

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}
