package animation

import (
	"errors"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/google/go-cmp/cmp"
)

const twoStateDiagram = `@startuml
state "Initial" as s0
s0: count
state "Done" as s1
s1: result
[*] --> s0 : $count' = 0
s0 --> s1 : insert?coin ; $count >= 0 ; $result' is done
@enduml
`

// builtinsDiagram uses the builtins of the clause representation, which the
// old reader did not know.
const builtinsDiagram = `@startuml
state "Open" as s0
s0: m
s0: k
[*] --> s0 : @empty(m') ∧ @eq(k', 0)
s0 --> s0 : put?x ; @notin(m, x) ∧ @ne(x, k) ; @update(m', m, x, 1) ∧ @eq(k', x)
@enduml
`

func mustRead(t *testing.T, content string) *core.Diagram {
	t.Helper()
	diagram, err := coreio.ReadBytes([]byte(content))
	if err != nil {
		t.Fatalf("coreio.ReadBytes() error = %v", err)
	}
	return diagram
}

func newSession(t *testing.T, content string) *Session {
	t.Helper()
	session, err := NewSession(mustRead(t, content))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}

func TestNewSessionStartsInValuesModeAtInitial(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)

	// Execute
	pending := session.Pending()

	// Assert
	if session.Mode() != ModeValues {
		t.Errorf("Mode() = %v, want ModeValues", session.Mode())
	}
	if diff := cmp.Diff(Target{ID: "s0", Name: "Initial", Vars: []string{"count"}}, pending.Target); diff != "" {
		t.Errorf("pending target mismatch (-want +got):\n%s", diff)
	}
	if len(pending.Guard.Conjuncts) != 0 || pending.Prev != nil {
		t.Errorf("pending = %+v, want no guard and no previous state", pending)
	}
	if got := ClauseText(pending.Post); got != "$count' = 0" {
		t.Errorf("pending post = %q, want %q", got, "$count' = 0")
	}
}

func TestNewSessionMissingInitialStateErrors(t *testing.T) {
	// Execute
	_, err := NewSession(&core.Diagram{Init: "missing"})

	// Assert
	if err == nil {
		t.Fatal("NewSession() error = nil, want error")
	}
}

func TestEnterValuesInitialAdvancesToCommand(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)

	// Execute
	state, err := session.EnterValues("[0]")

	// Assert
	if err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	want := State{ID: "s0", Name: "Initial", Values: []Value{{Name: "count", Value: jsonNumber("0")}}}
	if diff := cmp.Diff(want, state); diff != "" {
		t.Errorf("EnterValues() mismatch (-want +got):\n%s", diff)
	}
	if session.Mode() != ModeCommand {
		t.Errorf("Mode() = %v, want ModeCommand", session.Mode())
	}
	if got := session.Trace(); len(got) != 0 {
		t.Errorf("Trace() = %v, want empty (tau is not recorded)", got)
	}
	if got := session.History(); len(got) != 1 {
		t.Errorf("len(History()) = %d, want 1", len(got))
	}
}

func TestEnterValuesRefusesValuesThatDoNotFitTheTarget(t *testing.T) {
	tests := map[string]struct {
		encoded string
		want    string
	}{
		"not JSON":        {encoded: "[", want: "invalid JSON array: unexpected EOF"},
		"not an array":    {encoded: "{}", want: "invalid JSON array: top-level value must be an array"},
		"a null":          {encoded: "[null]", want: "null is not a valid state variable value"},
		"too few values":  {encoded: "[]", want: "expected 1 value(s) for [count], got 0"},
		"too many values": {encoded: "[1, 2]", want: "expected 1 value(s) for [count], got 2"},
		"trailing data":   {encoded: "[1] [2]", want: "unexpected trailing data after JSON array"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := newSession(t, twoStateDiagram)

			// Execute
			_, err := session.EnterValues(tt.encoded)

			// Assert
			var refused *RefusedValuesError
			if !errors.As(err, &refused) || refused.Error() != tt.want {
				t.Fatalf("EnterValues() error = %v, want refused values saying %q", err, tt.want)
			}
			if session.Mode() != ModeValues || len(session.History()) != 0 {
				t.Errorf("the session moved: mode %v, %d history entries", session.Mode(), len(session.History()))
			}
		})
	}
}

func TestEnterValuesInCommandModeErrors(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	// Execute
	_, err := session.EnterValues("[1]")

	// Assert
	var refused *RefusedValuesError
	if err == nil || errors.As(err, &refused) {
		t.Errorf("EnterValues() in command mode error = %v, want an error that is no refusal", err)
	}
}

func TestSelectThenEnterValuesRecordsEventTrace(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	// Execute
	if err := session.Select(0); err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	pending := session.Pending()
	if _, err := session.EnterValues(`["ok"]`); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	// Assert
	if pending.Target.ID != "s1" || pending.Prev == nil || pending.Prev.ID != "s0" || pending.Event.Label() != "insert?coin" {
		t.Errorf("pending = %+v, want s0 --insert?coin--> s1", pending)
	}
	if diff := cmp.Diff([]string{"insert?coin"}, session.Trace()); diff != "" {
		t.Errorf("Trace() mismatch (-want +got):\n%s", diff)
	}
}

func TestADiagramWithTheNewBuiltinsSteps(t *testing.T) {
	// Setup
	session := newSession(t, builtinsDiagram)

	// Execute
	_, err1 := session.EnterValues(`[{}, 0]`)
	err2 := session.Select(0)
	pending := session.Pending()
	state, err3 := session.EnterValues(`[{"1": 1}, 1]`)

	// Assert
	if err := errors.Join(err1, err2, err3); err != nil {
		t.Fatal(err)
	}
	if got := ClauseText(pending.Post); got != "@update(m', m, x, 1) ∧ @eq(k', x)" {
		t.Errorf("pending post = %q", got)
	}
	want := State{ID: "s0", Name: "Open", Values: []Value{
		{Name: "m", Value: map[string]any{"1": jsonNumber("1")}},
		{Name: "k", Value: jsonNumber("1")},
	}}
	if diff := cmp.Diff(want, state); diff != "" {
		t.Errorf("EnterValues() mismatch (-want +got):\n%s", diff)
	}
}

func TestSelectOutOfRangeErrors(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	// Execute
	err := session.Select(5)

	// Assert
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("Select(5) error = %v, want ErrIndexOutOfRange", err)
	}
}

func TestSelectInValuesModeErrors(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)

	// Execute
	err := session.Select(0)

	// Assert
	if err == nil {
		t.Error("Select() in values mode error = nil, want error")
	}
}

func TestJumpClonesHistoryEntry(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	_, err1 := session.EnterValues("[0]")
	err2 := session.Select(0)
	_, err3 := session.EnterValues(`["ok"]`)
	if err := errors.Join(err1, err2, err3); err != nil {
		t.Fatal(err)
	}

	// Execute
	err := session.Jump(0)

	// Assert
	if err != nil {
		t.Fatalf("Jump() error = %v", err)
	}
	if len(session.History()) != 3 {
		t.Errorf("len(History()) = %d, want 3", len(session.History()))
	}
	current, ok := session.Current()
	if !ok || current.ID != "s0" {
		t.Errorf("Current() = (%v, %v), want s0 in command mode", current, ok)
	}
	if len(session.Trace()) != 0 {
		t.Errorf("Trace() = %v, want empty after jumping to initial", session.Trace())
	}
}

func TestJumpOutOfRangeErrors(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	// Execute
	err := session.Jump(9)

	// Assert
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("Jump(9) error = %v, want ErrIndexOutOfRange", err)
	}
}

func TestBackReturnsToLastHistoryState(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	_, err1 := session.EnterValues("[0]")
	err2 := session.Select(0)
	if err := errors.Join(err1, err2); err != nil {
		t.Fatal(err)
	}

	// Execute
	err := session.Back()

	// Assert
	if err != nil {
		t.Fatalf("Back() error = %v", err)
	}
	current, ok := session.Current()
	if !ok || current.ID != "s0" {
		t.Errorf("Current() = (%v, %v), want s0 in command mode", current, ok)
	}
}

func TestBackWithoutHistoryErrors(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)

	// Execute
	err := session.Back()

	// Assert
	if err == nil {
		t.Error("Back() with no history error = nil, want error")
	}
}

func TestTransitionsListsOutgoingEdges(t *testing.T) {
	// Setup
	session := newSession(t, twoStateDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	// Execute
	edges := session.Transitions()

	// Assert
	if len(edges) != 1 || edges[0].Event.Label() != "insert?coin" {
		t.Errorf("Transitions() = %v, want one insert?coin edge", edges)
	}
}
