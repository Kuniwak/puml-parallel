package animation

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// tau is the silent/internal event. It is never appended to a trace, so the
// initial transition (which uses tau) produces an empty trace.
const tau csdf.Event = "tau"

// ErrIndexOutOfRange is returned by Select and Jump when the given index is not
// a valid transition or history index. Callers can distinguish this recoverable
// condition from fatal errors with errors.Is.
var ErrIndexOutOfRange = errors.New("Index out of range")

// Mode reports whether the session is awaiting state-variable values for a
// pending transition (ModeValues) or ready for a command on the current state
// (ModeCommand).
type Mode int

const (
	ModeValues Mode = iota
	ModeCommand
)

// pendingTransition is the transition whose post state group is awaiting values.
type pendingTransition struct {
	group csdf.State
	guard string
	post  string
	event csdf.Event
	prev  *csdf.RuntimeState
}

// Session is a single in-memory exploration of a diagram. It is not safe for
// concurrent use; callers that share a session must serialize access.
type Session struct {
	diagram *csdf.Diagram
	solver  csdf.PostSolver
	history []HistoryEntry
	mode    Mode
	pending pendingTransition
	current csdf.RuntimeState
}

// NewSession starts an exploration at the diagram's initial state. The returned
// session is in ModeValues, awaiting the initial state group's values.
func NewSession(diagram *csdf.Diagram, solver csdf.PostSolver) (*Session, error) {
	initial, ok := diagram.States[diagram.StartEdge.Dst]
	if !ok {
		return nil, fmt.Errorf("animation.NewSession: initial state %q does not exist", diagram.StartEdge.Dst)
	}
	return &Session{
		diagram: diagram,
		solver:  solver,
		mode:    ModeValues,
		pending: pendingTransition{
			group: initial,
			guard: csdf.True,
			post:  diagram.StartEdge.Post,
			event: tau,
			prev:  nil,
		},
	}, nil
}

// Diagram returns the diagram being explored.
func (s *Session) Diagram() *csdf.Diagram { return s.diagram }

// Mode reports the current mode.
func (s *Session) Mode() Mode { return s.mode }

// Pending returns the post state group awaiting values together with its guard,
// post condition, triggering event, and the previous state. Meaningful only in
// ModeValues.
func (s *Session) Pending() (group csdf.State, guard, post string, event csdf.Event, prev *csdf.RuntimeState) {
	p := s.pending
	return p.group, p.guard, p.post, p.event, p.prev
}

// Current returns the current state and whether the session is in ModeCommand.
func (s *Session) Current() (csdf.RuntimeState, bool) {
	return s.current, s.mode == ModeCommand
}

// Transitions returns the outgoing transitions from the current state.
func (s *Session) Transitions() []csdf.Edge {
	return Outgoing(s.diagram, s.current.ID)
}

// History returns the explored history entries.
func (s *Session) History() []HistoryEntry { return s.history }

// Trace returns the event trace of the current path.
func (s *Session) Trace() []csdf.Event { return s.currentTrace() }

// EnterValues resolves the entered values against the pending post state group.
// On PostSolverResultOK it appends a history entry, sets the current state, and
// switches to ModeCommand; otherwise the session is unchanged. The returned
// PostSolverResult carries the kind, the new state, and any syntax error for the
// caller to report. The error is non-nil only when the session is not awaiting
// values; callers should check it before inspecting the result.
func (s *Session) EnterValues(encoded string) (csdf.PostSolverResult, error) {
	if s.mode != ModeValues {
		return csdf.PostSolverResult{}, errors.New("animation.Session.EnterValues: not awaiting values")
	}

	result := s.solver(csdf.PostSolverInput{
		StateGroup:    s.pending.group,
		Previous:      s.pending.prev,
		Guard:         s.pending.guard,
		Post:          s.pending.post,
		EncodedValues: encoded,
	})
	if result.Kind != csdf.PostSolverResultOK {
		return result, nil
	}

	trace := append([]csdf.Event{}, s.currentTrace()...)
	if s.pending.event != tau {
		trace = append(trace, s.pending.event)
	}
	s.history = append(s.history, HistoryEntry{State: result.State, Trace: trace})
	s.current = result.State
	s.mode = ModeCommand
	return result, nil
}

// Select chooses the idx-th outgoing transition of the current state and
// switches to ModeValues, awaiting the destination group's values.
func (s *Session) Select(idx int) error {
	if s.mode != ModeCommand {
		return errors.New("animation.Session.Select: not in command mode")
	}
	edges := Outgoing(s.diagram, s.current.ID)
	if idx < 0 || idx >= len(edges) {
		return ErrIndexOutOfRange
	}
	edge := edges[idx]
	next, ok := s.diagram.States[edge.Dst]
	if !ok {
		return fmt.Errorf("animation.Session.Select: destination state %q does not exist", edge.Dst)
	}
	prev := s.current
	s.pending = pendingTransition{
		group: next,
		guard: edge.Guard,
		post:  edge.Post,
		event: edge.Event,
		prev:  &prev,
	}
	s.mode = ModeValues
	return nil
}

// Jump branches from the idx-th history entry: a clone is appended as a new
// entry and becomes the current state, preserving the linear history.
func (s *Session) Jump(idx int) error {
	if s.mode != ModeCommand {
		return errors.New("animation.Session.Jump: not in command mode")
	}
	if idx < 0 || idx >= len(s.history) {
		return ErrIndexOutOfRange
	}
	entry := cloneHistoryEntry(s.history[idx])
	s.history = append(s.history, entry)
	s.current = entry.State
	return nil
}

// Back discards the pending transition and returns to ModeCommand at the most
// recently explored state. It errors when there is no history to return to.
func (s *Session) Back() error {
	if len(s.history) == 0 {
		return errors.New("animation.Session.Back: no history")
	}
	s.current = s.history[len(s.history)-1].State
	s.mode = ModeCommand
	return nil
}

func (s *Session) currentTrace() []csdf.Event {
	if len(s.history) == 0 {
		return nil
	}
	return s.history[len(s.history)-1].Trace
}

// Outgoing returns the diagram's edges whose source is stateID, in declaration
// order.
func Outgoing(diagram *csdf.Diagram, stateID csdf.StateID) []csdf.Edge {
	var edges []csdf.Edge
	for _, edge := range diagram.Edges {
		if edge.Src == stateID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func cloneHistoryEntry(entry HistoryEntry) HistoryEntry {
	entry.State.Values = append([]csdf.StateValue{}, entry.State.Values...)
	entry.Trace = append([]csdf.Event{}, entry.Trace...)
	return entry
}
