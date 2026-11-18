// Package proto is the remote API for driving animation sessions: the request
// and response message contract, JSONL framing, the server-side request handler
// (Service), and a client round-trip stub (Do). It is transport-agnostic so a
// Unix-socket daemon and a future HTTP/WebSocket server can both reuse it.
package proto

import (
	"encoding/json"

	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
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
	CommandBack          = "back"
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
	// Code names the kind of the error, one of the ErrorCode constants, for a
	// client that words it itself; "" when the kind has no name. Args are
	// the parts of the error such a client puts in its wording.
	Code string            `json:"code,omitempty"`
	Args map[string]string `json:"args,omitempty"`
}

// Error codes carried by Response.Code.
const (
	// ErrorCodeRefusedValues: the values do not fit the state, or the
	// checker does not admit them; Args["reason"] says why. A refusal
	// whose kind the engine names (animation.RefusedValuesError.Code)
	// carries that code instead, its particulars beside "reason" in Args.
	ErrorCodeRefusedValues = "refused_values"
	// ErrorCodeIndexOutOfRange: no transition, end edge or history entry
	// has the index.
	ErrorCodeIndexOutOfRange = "index_out_of_range"
	// ErrorCodeEnded: the run has ended; only a jump goes on.
	ErrorCodeEnded = "ended"
)

// View is the structured current position of a session, returned by read,
// select, statevar (on success), jump and end. In "ended" mode, State is
// where the run ended.
type View struct {
	Mode        string           `json:"mode"` // "values" | "command" | "ended"
	State       *animation.State `json:"state,omitempty"`
	Transitions []Transition     `json:"transitions,omitempty"`
	Ends        []End            `json:"ends,omitempty"`
	Pending     *Pending         `json:"pending,omitempty"`
	// Families are, for a diagram expanded from a promotion, where the
	// instances of each family are in State.
	Families []FamilyView `json:"families,omitempty"`
}

// FamilyView is where the instances of the family of Map are: the IDs of
// those at each local state, sorted, a state no instance is at left out.
// Block is the <<promote>> block that draws the family in State's mode, ""
// when there is none.
type FamilyView struct {
	Map       string              `json:"map"`
	Block     string              `json:"block,omitempty"`
	Instances map[string][]string `json:"instances"`
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
	// Line is where the reader read the edge, as core.Edge.Line is: 0 when
	// it has none, as in an expansion.
	Line int `json:"line,omitempty"`
	// Moves are, for a diagram expanded from a promotion, the local edges
	// the transition is made of; none for a controller's own edge.
	Moves []Move `json:"moves,omitempty"`
}

// Move is one local edge a transition of a promoted diagram takes: the
// LocalEdge-th edge of the local diagram of the family of Map, from the
// local state Src to Dst, the Parallel-th of the local edges from Src to
// Dst, as the <<promote>> block Block draws it in the mode the transition
// leaves ("" when the reader knows no block). Kind is "stay", "creation" or
// "deletion": what the move does to its instance.
type Move struct {
	Map       string `json:"map"`
	Block     string `json:"block,omitempty"`
	LocalEdge int    `json:"local_edge"`
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	Parallel  int    `json:"parallel"`
	Kind      string `json:"kind"`
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
	// SuggestError is why the checker could not suggest values, when it
	// failed.
	SuggestError string `json:"suggest_error,omitempty"`
	// Determined are the values the post fixes by builtins alone
	// (animation.Determined), which a client may enter without asking.
	Determined []animation.Value `json:"determined,omitempty"`
	// Families are the variables of Group that hold the instances of a
	// family, in declaration order: what a client words in terms of the
	// local diagrams.
	Families []string `json:"families,omitempty"`
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
