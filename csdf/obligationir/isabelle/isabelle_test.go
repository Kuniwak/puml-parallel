package isabelle

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCompileTauSelfLoopWithVars(t *testing.T) {
	// A guarded tau self-loop carrying a variable becomes: the datatype, the
	// guard/post as True-placeholder definitions (each preceded by its
	// natural-language text), the tau_step relation, and the livelock_free theorem
	// left as oops.
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
a: n ; nat
[*] --> a
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

(* n > 0 *)
definition pred_1gdozh4 :: "val \<Rightarrow> bool"
  where "pred_1gdozh4 n \<equiv> True"

(* n' = n - 1 *)
definition pred_1nuhmrf :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "pred_1nuhmrf n n' \<equiv> True"

(* n > 0 *)
definition guard_L5 :: "val \<Rightarrow> bool"
  where "guard_L5 \<equiv> pred_1gdozh4"

(* n' = n - 1 *)
definition post_L5 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_L5 \<equiv> pred_1nuhmrf"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
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

(* n' = n - 1 *)
definition pred_7ydc3w :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "pred_7ydc3w n n' \<equiv> True"

(* n > 0 *)
definition pred_1e81hjg :: "val \<Rightarrow> bool"
  where "pred_1e81hjg n \<equiv> True"

(* n > 0 *)
definition guard_L5 :: "val \<Rightarrow> bool"
  where "guard_L5 \<equiv> pred_1e81hjg"

(* n' = n - 1 *)
definition post_L5 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_L5 \<equiv> pred_7ydc3w"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_L5 n \<and> post_L5 n n'"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileMultipleTauEdgesAreParenthesisedDisjuncts(t *testing.T) {
	// Two tau edges become a parenthesised disjunction inside the where-clause so
	// neither existential captures the other's clause.
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

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> (s = a \<and> s' = b \<and> guard_L5 \<and> post_L5)
    \<or> (s = b \<and> s' = a \<and> guard_L6 \<and> post_L6)"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops

end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}
