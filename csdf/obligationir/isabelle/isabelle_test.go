package isabelle

import (
	"bytes"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/google/go-cmp/cmp"
)

func compile(t *testing.T, input string) string {
	t.Helper()
	d, err := csdf.ParseDiagram([]byte(input))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	var buf bytes.Buffer
	if err := Compile(&buf, obligationir.BuildObligationIR(d)); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return buf.String()
}

func TestCompileTauSelfLoopWithVars(t *testing.T) {
	// A guarded tau self-loop carrying a variable becomes: the datatype, the
	// guard/post as True-placeholder definitions (each preceded by its
	// natural-language text), the tau_step relation, and the livelock_free theorem
	// left as oops.
	got := compile(t, `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `theory Livelock_Obligation imports Main begin
(* structurally_livelock_free: false *)
datatype json = JSONInt int | JSONString string | JSONBool bool | JSONArray "json list" | JSONDict "(string × json) list"
datatype st =
    a json (* declared: Nat *)

(* "n > 0" *)
definition Guard_L5 :: "json ⇒ bool" where "Guard_L5 n ≡ True"
(* "n' = n - 1" *)
definition Post_L5 :: "json ⇒ json ⇒ bool" where "Post_L5 n n' ≡ True"

definition tau_step :: "st ⇒ st ⇒ bool" where
  "tau_step s s' ≡ ∃n n'. s = a n ∧ s' = a n' ∧ Guard_L5 n ∧ Post_L5 n n'"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops
end
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileStructurallyFreeHasFalseRelation(t *testing.T) {
	// A visible-only chain has no tau edge, so tau_step is False and no predicate
	// definitions are emitted.
	got := compile(t, `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	want := `theory Livelock_Obligation imports Main begin
(* structurally_livelock_free: true *)
datatype st =
    s0
  | s1

definition tau_step :: "st ⇒ st ⇒ bool" where
  "tau_step s s' ≡ False"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops
end
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileUntypedVariableIsJson(t *testing.T) {
	// An untyped state variable is still a json value; no declared-type comment is
	// emitted because nothing was declared.
	got := compile(t, `@startuml
state "a" as a
a: n
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `theory Livelock_Obligation imports Main begin
(* structurally_livelock_free: false *)
datatype json = JSONInt int | JSONString string | JSONBool bool | JSONArray "json list" | JSONDict "(string × json) list"
datatype st =
    a json

(* "n > 0" *)
definition Guard_L5 :: "json ⇒ bool" where "Guard_L5 n ≡ True"
(* "n' = n - 1" *)
definition Post_L5 :: "json ⇒ json ⇒ bool" where "Post_L5 n n' ≡ True"

definition tau_step :: "st ⇒ st ⇒ bool" where
  "tau_step s s' ≡ ∃n n'. s = a n ∧ s' = a n' ∧ Guard_L5 n ∧ Post_L5 n n'"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops
end
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileMultipleTauEdgesAreParenthesisedDisjuncts(t *testing.T) {
	// Two tau edges become a parenthesised disjunction inside the where-clause so
	// neither existential captures the other's clause.
	got := compile(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)

	want := `theory Livelock_Obligation imports Main begin
(* structurally_livelock_free: false *)
datatype st =
    a
  | b

definition tau_step :: "st ⇒ st ⇒ bool" where
  "tau_step s s' ≡
    (s = a ∧ s' = b ∧ True ∧ True)
    ∨ (s = b ∧ s' = a ∧ True ∧ True)"

theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops
end
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}
