// Package proto is the remote API for driving animation sessions: the request
// and response message contract, JSONL framing, the server-side request handler
// (Service), and a client round-trip stub (Do). It is transport-agnostic so a
// Unix-socket daemon and a future HTTP/WebSocket server can both reuse it.
package proto

import (
	"encoding/json"

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
	CommandEnd           = "end"
	CommandReplay        = "replay"
	CommandServerVersion = "server_version"
)

// Request is a single command sent from the client to the service.
type Request struct {
	Command string `json:"command"`
	Session string `json:"session,omitempty"` // "" => resolve the single session
	Path    string `json:"path,omitempty"`    // session_new: label for listings
	Content []byte `json:"content,omitempty"` // session_new: diagram bytes (base64)
	Index   *int   `json:"index,omitempty"`   // select (optional) / jump, end (required)
	Values  string `json:"values,omitempty"`  // statevar: JSON-array text
	// Pick enters the N-th suggestion of the pending prompt instead of Values
	// (statevar), when the daemon has a checker to suggest from.
	Pick *int `json:"pick,omitempty"`
	// Includes (session_new) is the source of every file a promotion in
	// Content includes, by the path its !include writes; the daemon reads
	// no file of its own.
	Includes map[string]string `json:"includes,omitempty"`
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
	// Warnings (session_new) are what the promotion the session explores
	// warns of, one per line, for the client to print on its standard error.
	Warnings []string `json:"warnings,omitempty"`
}

// View is the structured current position of a session, returned by read,
// select, statevar (on success), jump and end. In "ended" mode, State is
// where the run ended.
type View struct {
	Mode        string           `json:"mode"` // "values" | "command" | "ended"
	State       *animation.State `json:"state,omitempty"`
	Transitions []Transition     `json:"transitions,omitempty"`
	Ends        []End            `json:"ends,omitempty"`
	Pending     *Pending         `json:"pending,omitempty"`
}

// End is one end edge from the current state, which end takes: its guard
// as Transition writes one.
type End struct {
	Index int    `json:"index"`
	Guard string `json:"guard"`
}

// Transition is one selectable outgoing edge from the current state: its
// event's label, and its guard and post as the canonical clause printer
// writes them, as animation.ClauseText does: "(none)" for a clause of no
// conjunct, as in the text output.
type Transition struct {
	Index   int    `json:"index"`
	Event   string `json:"event"`
	Dst     string `json:"dst"`
	DstName string `json:"dst_name"`
	Guard   string `json:"guard"`
	Post    string `json:"post"`
}

// Pending describes the state awaiting values in ModeValues, with the guard
// and post of the transition to it as Transition writes them.
type Pending struct {
	Previous *animation.State `json:"previous,omitempty"`
	Group    animation.Target `json:"group"`
	Guard    string           `json:"guard"`
	Post     string           `json:"post"`
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
	Trace []string `json:"trace"`
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
	Session   string `json:"session"`
	Path      string `json:"path,omitempty"`
	Mode      string `json:"mode"`
	StateID   string `json:"state_id"`
	StateName string `json:"state_name"`
}

func mustData(v any) json.RawMessage {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return encoded
}
