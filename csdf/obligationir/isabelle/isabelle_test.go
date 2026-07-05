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
a: n ; Nat
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

datatype st = a val (* declared: (n :: Nat) *)

(* n > 0 *)
definition guard_1tjwfxe :: "val \<Rightarrow> bool"
  where "guard_1tjwfxe n \<equiv> True"

(* n' = n - 1 *)
definition post_101s3ia :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_101s3ia n n' \<equiv> True"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_1tjwfxe n \<and> post_101s3ia n n'"

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

datatype st = s0
            | s1

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> False"

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

datatype st = a val (* declared: (n :: any) *)

(* n > 0 *)
definition guard_13qjgd2 :: "val \<Rightarrow> bool"
  where "guard_13qjgd2 n \<equiv> True"

(* n' = n - 1 *)
definition post_en24r7 :: "val \<Rightarrow> val \<Rightarrow> bool"
  where "post_en24r7 n n' \<equiv> True"

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv> \<exists>n n'. s = a n \<and> s' = a n' \<and> guard_13qjgd2 n \<and> post_en24r7 n n'"

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

definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where "tau_step s s' \<equiv>
    (s = a \<and> s' = b \<and> True \<and> True)
    \<or> (s = b \<and> s' = a \<and> True \<and> True)"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops
end
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}
