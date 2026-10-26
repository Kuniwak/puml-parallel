package animation

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

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

// Pending is the transition whose target state awaits values: the start
// edge (Prev nil, Event tau, no guard) or the edge last selected.
type Pending struct {
	Target Target
	Guard  clause.And
	Post   clause.And
	Event  core.Event
	Prev   *State
	// edge is the index of the edge among Prev's outgoing ones; -1 for
	// the start edge.
	edge int
}

// Session is a single in-memory exploration of a diagram. It is not safe for
// concurrent use; callers that share a session must serialize access.
type Session struct {
	diagram *core.Diagram
	history []HistoryEntry
	mode    Mode
	pending Pending
	current State
}

// NewSession starts an exploration at the diagram's initial state. The returned
// session is in ModeValues, awaiting the initial state's values.
func NewSession(diagram *core.Diagram) (*Session, error) {
	if !slices.Contains(diagram.States, diagram.Init) {
		return nil, fmt.Errorf("animation.NewSession: initial state %q does not exist", diagram.Init)
	}
	return &Session{
		diagram: diagram,
		mode:    ModeValues,
		pending: Pending{
			Target: targetOf(diagram, diagram.Init),
			Post:   diagram.InitPost,
			Event:  core.Event{Channel: core.Tau},
			edge:   -1,
		},
	}, nil
}

// Diagram returns the diagram being explored.
func (s *Session) Diagram() *core.Diagram { return s.diagram }

// Mode reports the current mode.
func (s *Session) Mode() Mode { return s.mode }

// Pending returns the transition awaiting values. Meaningful only in
// ModeValues.
func (s *Session) Pending() Pending { return s.pending }

// Current returns the current state and whether the session is in ModeCommand.
func (s *Session) Current() (State, bool) {
	return s.current, s.mode == ModeCommand
}

// Transitions returns the outgoing transitions from the current state.
func (s *Session) Transitions() []core.Edge {
	return Outgoing(s.diagram, s.current.ID)
}

// History returns the explored history entries.
func (s *Session) History() []HistoryEntry { return s.history }

// Trace returns the event labels of the current path, taus left out.
func (s *Session) Trace() []string { return s.currentTrace() }

// EnterValues binds the entered values, a JSON array in declaration order,
// to the pending target's variables. On success it appends a history entry,
// sets the current state, and switches to ModeCommand. Values that do not
// fit the target are a *RefusedValuesError, and the session is unchanged;
// any other error says the session is not awaiting values.
func (s *Session) EnterValues(encoded string) (State, error) {
	if s.mode != ModeValues {
		return State{}, errors.New("animation.Session.EnterValues: not awaiting values")
	}
	values, err := decodeValues(s.diagram, s.pending.Target, encoded)
	if err != nil {
		return State{}, err
	}
	state := State{ID: s.pending.Target.ID, Name: s.pending.Target.Name, Values: values}

	trace := slices.Clone(s.currentTrace())
	if s.pending.Event.Channel != core.Tau {
		trace = append(trace, s.pending.Event.Label())
	}
	if trace == nil {
		trace = []string{}
	}
	s.history = append(s.history, HistoryEntry{State: state, Trace: trace})
	s.current = state
	s.mode = ModeCommand
	return state, nil
}

// Select chooses the idx-th outgoing transition of the current state and
// switches to ModeValues, awaiting the destination's values.
func (s *Session) Select(idx int) error {
	if s.mode != ModeCommand {
		return errors.New("animation.Session.Select: not in command mode")
	}
	edges := Outgoing(s.diagram, s.current.ID)
	if idx < 0 || idx >= len(edges) {
		return ErrIndexOutOfRange
	}
	edge := edges[idx]
	prev := s.current
	s.pending = Pending{
		Target: targetOf(s.diagram, edge.Dst),
		Guard:  edge.Guard,
		Post:   edge.Post,
		Event:  edge.Event,
		Prev:   &prev,
		edge:   idx,
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

func (s *Session) currentTrace() []string {
	if len(s.history) == 0 {
		return nil
	}
	return s.history[len(s.history)-1].Trace
}

// Outgoing returns the diagram's edges whose source is stateID, in the
// diagram's order, which csdfbmc numbers them in too.
func Outgoing(diagram *core.Diagram, stateID string) []core.Edge {
	var edges []core.Edge
	for _, edge := range diagram.Edges {
		if edge.Src == stateID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func cloneHistoryEntry(entry HistoryEntry) HistoryEntry {
	entry.State.Values = slices.Clone(entry.State.Values)
	entry.Trace = slices.Clone(entry.Trace)
	return entry
}
