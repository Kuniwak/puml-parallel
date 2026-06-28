package lean

import (
	"bytes"
	"strings"
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
	// A guarded tau self-loop carrying a variable becomes: the state ADT, the
	// guard/post as True-placeholder defs (each preceded by its natural-language
	// text), the tau-step relation, and the livelock_free theorem left as sorry.
	got := compile(t, `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `-- structurally_livelock_free: false
inductive Json where
  | JSONInt (i : Int)
  | JSONString (s : String)
  | JSONBool (b : Bool)
  | JSONArray (xs : List Json)
  | JSONDict (kvs : List (String × Json))

inductive St where
  | a (n : Json) -- declared: Nat

-- "n > 0"
def Guard_L5 (n : Json) : Prop := True
-- "n' = n - 1"
def Post_L5 (n : Json) (n' : Json) : Prop := True

def tauStep (s s' : St) : Prop :=
  ∃ n n', s = .a n ∧ s' = .a n' ∧ Guard_L5 n ∧ Post_L5 n n'

theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by
  sorry
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileStructurallyFreeHasFalseRelation(t *testing.T) {
	// A visible-only chain has no tau edge, so the tau-step relation is False and
	// no predicate defs are emitted.
	got := compile(t, `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	want := `-- structurally_livelock_free: true
inductive St where
  | s0
  | s1

def tauStep (s s' : St) : Prop := False

theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by
  sorry
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileMultipleTauEdgesAreParenthesisedDisjuncts(t *testing.T) {
	// Two tau edges (a tau cycle over variable-free states) become a parenthesised
	// disjunction so neither existential captures the other's clause. Omitted
	// guard/post render as the literal True.
	got := compile(t, `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : tau
b --> a : tau
@enduml
`)

	want := `-- structurally_livelock_free: false
inductive St where
  | a
  | b

def tauStep (s s' : St) : Prop :=
  (s = .a ∧ s' = .b ∧ True ∧ True)
  ∨ (s = .b ∧ s' = .a ∧ True ∧ True)

theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by
  sorry
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileUntypedVariableIsJson(t *testing.T) {
	// An untyped state variable is still a Json value; no declared-type comment is
	// emitted because nothing was declared.
	got := compile(t, `@startuml
state "a" as a
a: n
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := `-- structurally_livelock_free: false
inductive Json where
  | JSONInt (i : Int)
  | JSONString (s : String)
  | JSONBool (b : Bool)
  | JSONArray (xs : List Json)
  | JSONDict (kvs : List (String × Json))

inductive St where
  | a (n : Json)

-- "n > 0"
def Guard_L5 (n : Json) : Prop := True
-- "n' = n - 1"
def Post_L5 (n : Json) (n' : Json) : Prop := True

def tauStep (s s' : St) : Prop :=
  ∃ n n', s = .a n ∧ s' = .a n' ∧ Guard_L5 n ∧ Post_L5 n n'

theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by
  sorry
`
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestCompileEscapesNewlineInPredicateText(t *testing.T) {
	// A multi-line natural-language predicate must stay on a single comment line.
	got := compile(t, `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") && strings.Contains(line, "\r") {
			t.Errorf("comment line contains a carriage return: %q", line)
		}
	}
}
