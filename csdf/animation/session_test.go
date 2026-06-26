package animation

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
)

const twoStateDiagram = `@startuml
state "Initial" as s0
s0: count ; number
state "Done" as s1
s1: result ; string
[*] --> s0 : count starts at zero
s0 --> s1 : insert(coin) ; count >= 0 ; result is done
@enduml
`

func mustParse(t *testing.T, content string) *csdf.Diagram {
	t.Helper()
	diagram, err := csdf.ParseDiagram([]byte(content))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	return diagram
}

func newTwoStateSession(t *testing.T) *Session {
	t.Helper()
	session, err := NewSession(mustParse(t, twoStateDiagram), csdf.SolveJSON)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}

// sequenceSolver yields the given results in order, plus a call counter.
func sequenceSolver(results ...csdf.PostSolverResult) (csdf.PostSolver, *int) {
	calls := 0
	return func(csdf.PostSolverInput) csdf.PostSolverResult {
		result := results[calls]
		calls++
		return result
	}, &calls
}

func TestNewSessionStartsInValuesModeAtInitial(t *testing.T) {
	session := newTwoStateSession(t)

	if session.Mode() != ModeValues {
		t.Errorf("Mode() = %v, want ModeValues", session.Mode())
	}
	group, guard, _, _, prev := session.Pending()
	if group.ID != "s0" {
		t.Errorf("pending group = %q, want s0", group.ID)
	}
	if guard != csdf.True {
		t.Errorf("pending guard = %q, want %q", guard, csdf.True)
	}
	if prev != nil {
		t.Errorf("pending prev = %v, want nil", prev)
	}
}

func TestNewSessionMissingInitialStateErrors(t *testing.T) {
	_, err := NewSession(&csdf.Diagram{StartEdge: csdf.StartEdge{Dst: "missing"}}, csdf.SolveJSON)
	if err == nil {
		t.Fatal("NewSession() error = nil, want error")
	}
}

func TestEnterValuesInitialAdvancesToCommand(t *testing.T) {
	session := newTwoStateSession(t)

	kind, state, err := session.EnterValues("[0]")
	if err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	if kind != csdf.PostSolverResultOK {
		t.Fatalf("EnterValues() kind = %v, want OK", kind)
	}
	if session.Mode() != ModeCommand {
		t.Errorf("Mode() = %v, want ModeCommand", session.Mode())
	}
	if state.ID != "s0" || len(state.Values) != 1 || state.Values[0].Name != "count" {
		t.Errorf("state = %+v, want s0 with count", state)
	}
	if got := session.Trace(); len(got) != 0 {
		t.Errorf("Trace() = %v, want empty (tau is not recorded)", got)
	}
	if got := session.History(); len(got) != 1 {
		t.Errorf("len(History()) = %d, want 1", len(got))
	}
}

func TestEnterValuesNonOKKeepsValuesMode(t *testing.T) {
	tests := []struct {
		name     string
		encoded  string
		wantKind csdf.PostSolverResultKind
	}{
		{name: "syntax error", encoded: "null", wantKind: csdf.PostSolverResultSyntaxError},
		{name: "length mismatch", encoded: "[]", wantKind: csdf.PostSolverResultInvalidStateVarValuesLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newTwoStateSession(t)
			kind, _, _ := session.EnterValues(tt.encoded)
			if kind != tt.wantKind {
				t.Errorf("EnterValues() kind = %v, want %v", kind, tt.wantKind)
			}
			if session.Mode() != ModeValues {
				t.Errorf("Mode() = %v, want ModeValues", session.Mode())
			}
			if len(session.History()) != 0 {
				t.Errorf("len(History()) = %d, want 0", len(session.History()))
			}
		})
	}
}

func TestEnterValuesInCommandModeErrors(t *testing.T) {
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	if _, _, err := session.EnterValues("[1]"); err == nil {
		t.Error("EnterValues() in command mode error = nil, want error")
	}
}

func TestSelectThenEnterValuesRecordsEventTrace(t *testing.T) {
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	if err := session.Select(0); err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if session.Mode() != ModeValues {
		t.Errorf("Mode() = %v, want ModeValues", session.Mode())
	}
	group, _, _, _, prev := session.Pending()
	if group.ID != "s1" {
		t.Errorf("pending group = %q, want s1", group.ID)
	}
	if prev == nil || prev.ID != "s0" {
		t.Errorf("pending prev = %v, want s0", prev)
	}

	if _, _, err := session.EnterValues(`["ok"]`); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	trace := session.Trace()
	if len(trace) != 1 || trace[0] != "insert(coin)" {
		t.Errorf("Trace() = %v, want [insert(coin)]", trace)
	}
}

func TestSelectOutOfRangeErrors(t *testing.T) {
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	if err := session.Select(5); err == nil || err.Error() != "Index out of range" {
		t.Errorf("Select(5) error = %v, want \"Index out of range\"", err)
	}
}

func TestSelectInValuesModeErrors(t *testing.T) {
	session := newTwoStateSession(t)
	if err := session.Select(0); err == nil {
		t.Error("Select() in values mode error = nil, want error")
	}
}

func TestJumpClonesHistoryEntry(t *testing.T) {
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	if err := session.Select(0); err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if _, _, err := session.EnterValues(`["ok"]`); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}

	if err := session.Jump(0); err != nil {
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
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	if err := session.Jump(9); err == nil || err.Error() != "Index out of range" {
		t.Errorf("Jump(9) error = %v, want \"Index out of range\"", err)
	}
}

func TestBackReturnsToLastHistoryState(t *testing.T) {
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	if err := session.Select(0); err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if err := session.Back(); err != nil {
		t.Fatalf("Back() error = %v", err)
	}
	if session.Mode() != ModeCommand {
		t.Errorf("Mode() = %v, want ModeCommand", session.Mode())
	}
	current, _ := session.Current()
	if current.ID != "s0" {
		t.Errorf("Current() = %q, want s0", current.ID)
	}
}

func TestBackWithoutHistoryErrors(t *testing.T) {
	session := newTwoStateSession(t)
	if err := session.Back(); err == nil {
		t.Error("Back() with no history error = nil, want error")
	}
}

func TestEnterValuesNoSolutionsUsesSolverKind(t *testing.T) {
	solver, calls := sequenceSolver(csdf.PostSolverResult{Kind: csdf.PostSolverResultNoSolutions})
	session, err := NewSession(mustParse(t, twoStateDiagram), solver)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	kind, _, _ := session.EnterValues("[0]")
	if kind != csdf.PostSolverResultNoSolutions {
		t.Errorf("EnterValues() kind = %v, want NoSolutions", kind)
	}
	if *calls != 1 {
		t.Errorf("solver calls = %d, want 1", *calls)
	}
	if session.Mode() != ModeValues {
		t.Errorf("Mode() = %v, want ModeValues", session.Mode())
	}
}

func TestTransitionsListsOutgoingEdges(t *testing.T) {
	session := newTwoStateSession(t)
	if _, _, err := session.EnterValues("[0]"); err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	edges := session.Transitions()
	if len(edges) != 1 || edges[0].Event != "insert(coin)" {
		t.Errorf("Transitions() = %v, want one insert(coin) edge", edges)
	}
}
