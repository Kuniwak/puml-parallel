package lean

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCompileTauSelfLoopWithVars(t *testing.T) {
	// A guarded tau self-loop carrying a variable becomes: the state inductive,
	// the guard/post as True-placeholder definitions (each preceded by its
	// natural-language text), the init predicate, the step and tauStep relations,
	// the inductive Reachable predicate, and the livelock_free theorem left as
	// sorry. The theorem is restricted to reachable states: over all of St it
	// would be strictly stronger than livelock freedom and often false.
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
a: n ; nat
[*] --> a : n = 10
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))

inductive St where
  | St_a (v_n : Val) -- type: (n : nat)

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n = 10
opaque pred_xjezwh : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n > 0
opaque pred_1gdozh4 : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n' = n - 1
opaque pred_1nuhmrf : Val → Val → Prop

-- n = 10
def init : Val → Prop := pred_xjezwh

-- n > 0
def guard_L5 : Val → Prop := pred_1gdozh4

-- n' = n - 1
def post_L5 : Val → Val → Prop := pred_1nuhmrf

def step (s s' : St) : Prop :=
  ∃ v_n v_n', s = .St_a v_n ∧ s' = .St_a v_n' ∧ guard_L5 v_n ∧ post_L5 v_n v_n'

inductive Reachable : St → Prop where
  | base (v_n : Val) : init v_n → Reachable (.St_a v_n)
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

def tauStep (s s' : St) : Prop :=
  ∃ v_n v_n', s = .St_a v_n ∧ s' = .St_a v_n' ∧ guard_L5 v_n ∧ post_L5 v_n v_n'

theorem livelock_free :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestSanitizeCommentCollapsesNewlines(t *testing.T) {
	// Unlike Isabelle's (* ... *), a Lean comment ends at the newline, so a
	// multi-line predicate text has to be folded onto one line.
	got := SanitizeComment("n > 0\r\nand n < 10")

	want := "n > 0 and n < 10"
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileStructurallyFreeEmitsNoObligation(t *testing.T) {
	// A visible-only chain has no tau edge, so — being structurally livelock free
	// — no declaration and no sorry obligation is emitted, only a note.
	got := MustCompileLivelockFreeString(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	want := `-- Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation.
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileUntypedVariableIsVal(t *testing.T) {
	// An untyped state variable is still a Val; the declared-type comment shows it
	// as "any".
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
a: n
[*] --> a : n = 10
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))

inductive St where
  | St_a (v_n : Val) -- type: (n : any)

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n' = n - 1
opaque pred_7ydc3w : Val → Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n = 10
opaque pred_icpx2l : Val → Prop

-- TODO(csdf): not formalised; this predicate is uninterpreted.
-- n > 0
opaque pred_1e81hjg : Val → Prop

-- n = 10
def init : Val → Prop := pred_icpx2l

-- n > 0
def guard_L5 : Val → Prop := pred_1e81hjg

-- n' = n - 1
def post_L5 : Val → Val → Prop := pred_7ydc3w

def step (s s' : St) : Prop :=
  ∃ v_n v_n', s = .St_a v_n ∧ s' = .St_a v_n' ∧ guard_L5 v_n ∧ post_L5 v_n v_n'

inductive Reachable : St → Prop where
  | base (v_n : Val) : init v_n → Reachable (.St_a v_n)
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

def tauStep (s s' : St) : Prop :=
  ∃ v_n v_n', s = .St_a v_n ∧ s' = .St_a v_n' ∧ guard_L5 v_n ∧ post_L5 v_n v_n'

theorem livelock_free :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompileMultipleTauEdgesAreParenthesisedDisjuncts(t *testing.T) {
	// Two tau edges become a parenthesised disjunction so neither existential
	// captures the other's clause. Variable-free states make every predicate the
	// same omitted "true", so init and both edges share one placeholder, and the
	// base rule takes no binder.
	got := MustCompileLivelockFreeString(`@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)

	want := `inductive St where
  | St_a
  | St_b

-- true
def pred_1ygzo25 : Prop := True

-- true
def init : Prop := pred_1ygzo25

-- true
def guard_L5 : Prop := pred_1ygzo25

-- true
def post_L5 : Prop := pred_1ygzo25

-- true
def guard_L6 : Prop := pred_1ygzo25

-- true
def post_L6 : Prop := pred_1ygzo25

def step (s s' : St) : Prop :=
  (s = .St_a ∧ s' = .St_b ∧ guard_L5 ∧ post_L5)
  ∨ (s = .St_b ∧ s' = .St_a ∧ guard_L6 ∧ post_L6)

inductive Reachable : St → Prop where
  | base : init → Reachable .St_a
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

def tauStep (s s' : St) : Prop :=
  (s = .St_a ∧ s' = .St_b ∧ guard_L5 ∧ post_L5)
  ∨ (s = .St_b ∧ s' = .St_a ∧ guard_L6 ∧ post_L6)

theorem livelock_free :
    WellFounded (fun s' s => Reachable s ∧ tauStep s s') := by
  sorry
`
	if want != got {
		t.Error(cmp.Diff(want, got))
	}
}
