package animation

import (
	"errors"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/google/go-cmp/cmp"
)

const twoStateDiagram = `@startuml
state "Initial" as s0
s0 : count ; count
state "Done" as s1
s1 : result ; result
[*] --> s0 : $count' = 0
s0 --> s1 : insert?coin ; $count >= 0 ; $result' is done
note as channels
  channel insert : coin
end note
@enduml
`

// builtinsDiagram uses the builtins of the clause representation, which the
// old reader did not know.
const builtinsDiagram = `@startuml
state "Open" as s0
s0 : m ; m
s0 : k ; k
[*] --> s0 : @empty(m') ∧ @eq(k', 0)
s0 --> s0 : put?x ; @notin(m, x) ∧ @ne(x, k) ; @update(m', m, x, 1) ∧ @eq(k', x)
note as channels
  channel put : x
end note
@enduml
`

// endingDiagram has a state whose only exit is an end edge.
const endingDiagram = `@startuml
state "Idle" as s0
s0 : a ; a
[*] --> s0 : $a' == 0
s0 --> [*] : @eq(a, a)
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
	session, err := NewSession(mustRead(t, content), NoSuggester{})
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
	// Setup
	diagram := &core.Diagram{Init: "missing"}

	// Execute
	_, err := NewSession(diagram, NoSuggester{})

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

// atInitial is a session of twoStateDiagram at its initial state, in
// command mode.
func atInitial(t *testing.T) *Session {
	t.Helper()
	session := newSession(t, twoStateDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("Setup: EnterValues() error = %v", err)
	}
	return session
}

func TestSelectOutOfRangeErrors(t *testing.T) {
	tests := map[string]int{
		"a negative index":       -1,
		"one past the last edge": 1,
		"far past the last edge": 5,
	}
	for name, idx := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := atInitial(t)

			// Execute
			err := session.Select(idx)

			// Assert
			if !errors.Is(err, ErrIndexOutOfRange) {
				t.Errorf("Select(%d) error = %v, want ErrIndexOutOfRange", idx, err)
			}
		})
	}
}

func TestJumpOutOfRangeErrors(t *testing.T) {
	tests := map[string]int{
		"a negative index":        -1,
		"one past the last entry": 1,
		"far past the last entry": 9,
	}
	for name, idx := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := atInitial(t)

			// Execute
			err := session.Jump(idx)

			// Assert
			if !errors.Is(err, ErrIndexOutOfRange) {
				t.Errorf("Jump(%d) error = %v, want ErrIndexOutOfRange", idx, err)
			}
		})
	}
}

func TestOperationsOutOfTheirModeError(t *testing.T) {
	tests := map[string]struct {
		session func(*testing.T) *Session
		op      func(*Session) error
	}{
		"select while awaiting values": {
			session: func(t *testing.T) *Session { return newSession(t, twoStateDiagram) },
			op:      func(s *Session) error { return s.Select(0) },
		},
		"enter values in command mode": {
			session: atInitial,
			op: func(s *Session) error {
				_, err := s.EnterValues("[1]")
				return err
			},
		},
		"back with no history": {
			session: func(t *testing.T) *Session { return newSession(t, twoStateDiagram) },
			op:      func(s *Session) error { return s.Back() },
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := tt.session(t)

			// Execute
			err := tt.op(session)

			// Assert
			var refused *RefusedValuesError
			if err == nil || errors.As(err, &refused) || errors.Is(err, ErrIndexOutOfRange) {
				t.Errorf("error = %v, want an error that is neither a refusal nor an index out of range", err)
			}
		})
	}
}

// endedAt is a session of endingDiagram whose run ended at its initial state.
func endedAt(t *testing.T) *Session {
	t.Helper()
	session := newSession(t, endingDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("Setup: EnterValues() error = %v", err)
	}
	if err := session.End(0); err != nil {
		t.Fatalf("Setup: End() error = %v", err)
	}
	return session
}

func TestEndTakesAnEndEdgeAndTheRunEnds(t *testing.T) {
	// Setup
	session := newSession(t, endingDiagram)
	if _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("Setup: EnterValues() error = %v", err)
	}

	// Execute
	err := session.End(0)

	// Assert
	if err != nil {
		t.Fatalf("End() error = %v", err)
	}
	if session.Mode() != ModeEnded {
		t.Errorf("Mode() = %v, want ModeEnded", session.Mode())
	}
	if current, ok := session.Current(); ok || current.ID != "s0" {
		t.Errorf("Current() = (%v, %v), want s0, not in command mode", current, ok)
	}
}

func TestEndRefusesAnEndEdgeABuiltinOfWhoseGuardIsFalse(t *testing.T) {
	// Setup
	session := newSession(t, "@startuml\nstate \"S\" as s0\ns0 : a ; a\ns0 : b ; b\n[*] --> s0 : @eq(a', b')\ns0 --> [*] : @ne(a, b)\n@enduml\n")
	if _, err := session.EnterValues("[1, 1]"); err != nil {
		t.Fatalf("Setup: EnterValues() error = %v", err)
	}

	// Execute
	err := session.End(0)

	// Assert
	var refused *RefusedValuesError
	want := "the guard does not hold of these values: @ne(a, b)"
	if !errors.As(err, &refused) || refused.Error() != want {
		t.Fatalf("End() error = %v, want a refusal saying %q", err, want)
	}
	if session.Mode() != ModeCommand {
		t.Errorf("Mode() = %v, want ModeCommand still", session.Mode())
	}
}

func TestEndOutOfRangeErrors(t *testing.T) {
	tests := map[string]int{
		"a negative index":           -1,
		"one past the last end edge": 1,
	}
	for name, idx := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := newSession(t, endingDiagram)
			if _, err := session.EnterValues("[0]"); err != nil {
				t.Fatalf("Setup: EnterValues() error = %v", err)
			}

			// Execute
			err := session.End(idx)

			// Assert
			if !errors.Is(err, ErrIndexOutOfRange) {
				t.Errorf("End(%d) error = %v, want ErrIndexOutOfRange", idx, err)
			}
		})
	}
}

func TestOperationsAfterTheRunEndedError(t *testing.T) {
	tests := map[string]func(*Session) error{
		"select": func(s *Session) error { return s.Select(0) },
		"end":    func(s *Session) error { return s.End(0) },
	}
	for name, op := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := endedAt(t)

			// Execute
			err := op(session)

			// Assert
			if !errors.Is(err, ErrEnded) {
				t.Errorf("error = %v, want ErrEnded", err)
			}
		})
	}
}

func TestJumpAfterTheRunEndedGoesOn(t *testing.T) {
	// Setup
	session := endedAt(t)

	// Execute
	err := session.Jump(0)

	// Assert
	if err != nil {
		t.Fatalf("Jump() error = %v", err)
	}
	if current, ok := session.Current(); !ok || current.ID != "s0" {
		t.Errorf("Current() = (%v, %v), want s0 in command mode", current, ok)
	}
}
