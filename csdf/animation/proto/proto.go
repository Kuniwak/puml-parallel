// Package proto is the remote API for driving animation sessions: the request
// and response message contract, JSONL framing, the server-side request handler
// (Service), and a client round-trip stub (Do). It is transport-agnostic so a
// Unix-socket daemon and a future HTTP/WebSocket server can both reuse it.
package proto

import (
	"encoding/json"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
)

// Command names carried by Request.Command.
const (
	CommandSessionNew    = "session_new"
	CommandSessionList   = "session_list"
	CommandSessionRm     = "session_rm"
	CommandRead          = "read"
	CommandSelect        = "select"
	CommandStatevar      = "statevar"
	CommandTrace         = "trace"
	CommandHistory       = "history"
	CommandJump          = "jump"
	CommandReplay        = "replay"
	CommandServerVersion = "server_version"
)

// Request is a single command sent from the client to the service.
type Request struct {
	Command string `json:"command"`
	Session string `json:"session,omitempty"` // "" => resolve the single session
	Path    string `json:"path,omitempty"`    // session_new: label for listings
	Content []byte `json:"content,omitempty"` // session_new: diagram bytes (base64)
	Index   *int   `json:"index,omitempty"`   // select (optional) / jump (required)
	Values  string `json:"values,omitempty"`  // statevar: JSON-array text
	// Pick enters the N-th suggestion of the pending prompt instead of Values
	// (statevar), when the daemon has a checker to suggest from.
	Pick *int `json:"pick,omitempty"`
	// DiagramJSON (session_new) is the JSON the checker reads for this session
	// when the file is an expansion; empty means the file's own parse.
	DiagramJSON string `json:"diagram_json,omitempty"`
	// Replay walks a fresh session along a path csdfbmc recorded under a
	// finding (replay).
	Replay *animation.Replay `json:"replay,omitempty"`
	// Environment (replay) is what the report the path comes from says
	// reproduces its run; nil when it says nothing.
	Environment *animation.Environment `json:"environment,omitempty"`
}

// Response is the service's reply. Output is the human-readable rendering; Data
// is the command-specific structured payload (see the *Data / View types).
type Response struct {
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Session string          `json:"session,omitempty"`
	Output  string          `json:"output,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// View is the structured current position of a session, returned by read,
// select, statevar (on success), and jump.
type View struct {
	Mode        string             `json:"mode"` // "values" | "command"
	State       *csdf.RuntimeState `json:"state,omitempty"`
	Transitions []Transition       `json:"transitions,omitempty"`
	Pending     *Pending           `json:"pending,omitempty"`
}

// Transition is one selectable outgoing edge from the current state.
type Transition struct {
	Index   int            `json:"index"`
	Event   csdf.Event     `json:"event"`
	Dst     csdf.StateID   `json:"dst"`
	DstName string         `json:"dst_name"`
	Guard   csdf.Predicate `json:"guard"`
	Post    csdf.Predicate `json:"post"`
}

// Pending describes the post state group awaiting values in ModeValues.
type Pending struct {
	Previous *csdf.RuntimeState `json:"previous,omitempty"`
	Group    csdf.StateWithID   `json:"group"`
	Guard    csdf.Predicate     `json:"guard"`
	Post     csdf.Predicate     `json:"post"`
	// Suggestions are the values the checker admits for this prompt, when
	// the daemon has a checker; nil otherwise.
	Suggestions []animation.Suggestion `json:"suggestions,omitempty"`
}

// SessionRef carries a single session id (session_new, session_rm).
type SessionRef struct {
	Session string `json:"session"`
}

// VersionData is the server_version payload.
type VersionData struct {
	Version string `json:"version"`
}

// TraceData is the trace payload.
type TraceData struct {
	Trace []csdf.Event `json:"trace"`
}

// HistoryData is the history payload.
type HistoryData struct {
	History []animation.HistoryEntry `json:"history"`
}

// SessionListData is the session_list payload.
type SessionListData struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo summarizes one live session.
type SessionInfo struct {
	Session   string       `json:"session"`
	Path      string       `json:"path,omitempty"`
	Mode      string       `json:"mode"`
	StateID   csdf.StateID `json:"state_id"`
	StateName string       `json:"state_name"`
}

func mustData(v any) json.RawMessage {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return encoded
}
