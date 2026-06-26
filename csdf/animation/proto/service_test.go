package proto

import (
	"encoding/json"
	"strings"
	"testing"
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

func intPtr(i int) *int { return &i }

// newSession creates a service holding one session at the initial value prompt
// and returns the service together with the new session id.
func newSession(t *testing.T) (*Service, string) {
	t.Helper()
	service := NewService("test-version", false)
	resp := service.Handle(Request{Command: CommandSessionNew, Path: "diagram.puml", Content: []byte(twoStateDiagram)})
	if !resp.OK {
		t.Fatalf("session_new failed: %s", resp.Error)
	}
	return service, resp.Session
}

// advance creates a service with one session already advanced into command mode
// at the initial state (count = 0).
func advance(t *testing.T) (*Service, string) {
	t.Helper()
	service, id := newSession(t)
	resp := service.Handle(Request{Command: CommandStatevar, Session: id, Values: "[0]"})
	if !resp.OK {
		t.Fatalf("statevar failed: %s", resp.Error)
	}
	return service, id
}

func decodeView(t *testing.T, resp Response) View {
	t.Helper()
	var view View
	if err := json.Unmarshal(resp.Data, &view); err != nil {
		t.Fatalf("decoding view: %v", err)
	}
	return view
}

func TestHandleSessionNewReturnsID(t *testing.T) {
	service := NewService("dev", false)
	resp := service.Handle(Request{Command: CommandSessionNew, Content: []byte(twoStateDiagram)})
	if !resp.OK {
		t.Fatalf("session_new failed: %s", resp.Error)
	}
	if resp.Session != "1" || resp.Output != "1\n" {
		t.Errorf("session_new = (session %q, output %q), want (\"1\", \"1\\n\")", resp.Session, resp.Output)
	}
	var ref SessionRef
	if err := json.Unmarshal(resp.Data, &ref); err != nil || ref.Session != "1" {
		t.Errorf("session_new data = %s (err %v), want session 1", resp.Data, err)
	}
}

func TestHandleSessionNewRejectsBadContent(t *testing.T) {
	service := NewService("dev", false)
	resp := service.Handle(Request{Command: CommandSessionNew, Content: []byte("not plantuml")})
	if resp.OK {
		t.Fatalf("session_new OK = true, want failure")
	}
}

func TestHandleReadShowsValuePromptInValuesMode(t *testing.T) {
	service, id := newSession(t)
	resp := service.Handle(Request{Command: CommandRead, Session: id})
	if !resp.OK {
		t.Fatalf("read failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Output, "Post State Group:") {
		t.Errorf("read output = %q, want value prompt", resp.Output)
	}
	view := decodeView(t, resp)
	if view.Mode != "values" || view.Pending == nil || view.Pending.Group.ID != "s0" {
		t.Errorf("read view = %+v, want values mode pending s0", view)
	}
}

func TestHandleStatevarAdvancesToCommand(t *testing.T) {
	service, id := newSession(t)
	resp := service.Handle(Request{Command: CommandStatevar, Session: id, Values: "[0]"})
	if !resp.OK {
		t.Fatalf("statevar failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Output, "State: Initial (s0)") || !strings.Contains(resp.Output, "Transitions:") {
		t.Errorf("statevar output = %q, want state + transitions", resp.Output)
	}
	view := decodeView(t, resp)
	if view.Mode != "command" || view.State == nil || view.State.ID != "s0" {
		t.Errorf("statevar view = %+v, want command mode at s0", view)
	}
	if len(view.Transitions) != 1 || view.Transitions[0].Event != "insert(coin)" {
		t.Errorf("statevar transitions = %+v, want one insert(coin)", view.Transitions)
	}
}

func TestHandleStatevarReportsSolverErrors(t *testing.T) {
	tests := []struct {
		name      string
		values    string
		wantError string
	}{
		{name: "syntax", values: "null", wantError: "top-level value must be an array"},
		{name: "length", values: "[]", wantError: "State variable values length mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, id := newSession(t)
			resp := service.Handle(Request{Command: CommandStatevar, Session: id, Values: tt.values})
			if resp.OK {
				t.Fatalf("statevar OK = true, want failure")
			}
			if !strings.Contains(resp.Error, tt.wantError) {
				t.Errorf("statevar error = %q, want substring %q", resp.Error, tt.wantError)
			}
		})
	}
}

func TestHandleStatevarErrorDebugSurfacesChain(t *testing.T) {
	newAt := func(debug bool) (*Service, string) {
		t.Helper()
		s := NewService("dev", debug)
		resp := s.Handle(Request{Command: CommandSessionNew, Path: "d.puml", Content: []byte(twoStateDiagram)})
		if !resp.OK {
			t.Fatalf("session_new failed: %s", resp.Error)
		}
		return s, resp.Session
	}

	// Default: deepest, prefix-free message.
	s, id := newAt(false)
	resp := s.Handle(Request{Command: CommandStatevar, Session: id, Values: "null"})
	if strings.Contains(resp.Error, "csdf.SolveJSON") {
		t.Errorf("default error %q leaks internal prefix", resp.Error)
	}

	// Debug: the full wrapped chain including the package-qualified context.
	sd, idd := newAt(true)
	respd := sd.Handle(Request{Command: CommandStatevar, Session: idd, Values: "null"})
	if !strings.Contains(respd.Error, "csdf.SolveJSON") {
		t.Errorf("debug error %q does not include the full chain", respd.Error)
	}
}

func TestHandleStatevarRejectedInCommandMode(t *testing.T) {
	service, id := advance(t)
	resp := service.Handle(Request{Command: CommandStatevar, Session: id, Values: "[0]"})
	if resp.OK {
		t.Fatalf("statevar OK = true, want failure in command mode")
	}
	if !strings.Contains(resp.Error, "not awaiting values") {
		t.Errorf("statevar error = %q, want \"not awaiting values\"", resp.Error)
	}
}

func TestHandleSelectMovesToValues(t *testing.T) {
	service, id := advance(t)
	resp := service.Handle(Request{Command: CommandSelect, Session: id, Index: intPtr(0)})
	if !resp.OK {
		t.Fatalf("select failed: %s", resp.Error)
	}
	view := decodeView(t, resp)
	if view.Mode != "values" || view.Pending == nil || view.Pending.Group.ID != "s1" {
		t.Errorf("select view = %+v, want values mode pending s1", view)
	}
}

func TestHandleSelectOutOfRange(t *testing.T) {
	service, id := advance(t)
	resp := service.Handle(Request{Command: CommandSelect, Session: id, Index: intPtr(9)})
	if resp.OK || resp.Error != "Index out of range" {
		t.Errorf("select(9) = (ok %v, error %q), want failure \"Index out of range\"", resp.OK, resp.Error)
	}
}

func TestHandleSelectWithoutIndexShowsView(t *testing.T) {
	service, id := advance(t)
	resp := service.Handle(Request{Command: CommandSelect, Session: id})
	if !resp.OK {
		t.Fatalf("select without index failed: %s", resp.Error)
	}
	view := decodeView(t, resp)
	if view.Mode != "command" || len(view.Transitions) != 1 {
		t.Errorf("select without index view = %+v, want command mode with transitions", view)
	}
}

func TestHandleTraceAfterSelectStatevar(t *testing.T) {
	service, id := advance(t)
	service.Handle(Request{Command: CommandSelect, Session: id, Index: intPtr(0)})
	service.Handle(Request{Command: CommandStatevar, Session: id, Values: `["ok"]`})

	resp := service.Handle(Request{Command: CommandTrace, Session: id})
	if !resp.OK {
		t.Fatalf("trace failed: %s", resp.Error)
	}
	var data TraceData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("decoding trace: %v", err)
	}
	if len(data.Trace) != 1 || data.Trace[0] != "insert(coin)" {
		t.Errorf("trace = %v, want [insert(coin)]", data.Trace)
	}
}

func TestHandleHistory(t *testing.T) {
	service, id := advance(t)
	resp := service.Handle(Request{Command: CommandHistory, Session: id})
	if !resp.OK {
		t.Fatalf("history failed: %s", resp.Error)
	}
	var data HistoryData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("decoding history: %v", err)
	}
	if len(data.History) != 1 {
		t.Errorf("history len = %d, want 1", len(data.History))
	}
}

func TestHandleJump(t *testing.T) {
	service, id := advance(t)
	service.Handle(Request{Command: CommandSelect, Session: id, Index: intPtr(0)})
	service.Handle(Request{Command: CommandStatevar, Session: id, Values: `["ok"]`})

	resp := service.Handle(Request{Command: CommandJump, Session: id, Index: intPtr(0)})
	if !resp.OK {
		t.Fatalf("jump failed: %s", resp.Error)
	}
	view := decodeView(t, resp)
	if view.Mode != "command" || view.State == nil || view.State.ID != "s0" {
		t.Errorf("jump view = %+v, want command mode at s0", view)
	}
}

func TestHandleJumpRequiresIndex(t *testing.T) {
	service, id := advance(t)
	resp := service.Handle(Request{Command: CommandJump, Session: id})
	if resp.OK || !strings.Contains(resp.Error, "requires a history index") {
		t.Errorf("jump without index = (ok %v, error %q), want failure", resp.OK, resp.Error)
	}
}

func TestSessionResolution(t *testing.T) {
	service, _ := newSession(t)

	// Exactly one session: empty session id resolves to it.
	if resp := service.Handle(Request{Command: CommandRead}); !resp.OK {
		t.Fatalf("read with single session failed: %s", resp.Error)
	}

	// A second session makes the empty id ambiguous.
	service.Handle(Request{Command: CommandSessionNew, Content: []byte(twoStateDiagram)})
	if resp := service.Handle(Request{Command: CommandRead}); resp.OK || !strings.Contains(resp.Error, "multiple sessions") {
		t.Errorf("read with two sessions = (ok %v, error %q), want \"multiple sessions\"", resp.OK, resp.Error)
	}

	// An unknown id is rejected.
	if resp := service.Handle(Request{Command: CommandRead, Session: "nope"}); resp.OK || !strings.Contains(resp.Error, "no such session") {
		t.Errorf("read unknown session = (ok %v, error %q), want \"no such session\"", resp.OK, resp.Error)
	}
}

func TestSessionListAndRemove(t *testing.T) {
	service, id := newSession(t)

	list := service.Handle(Request{Command: CommandSessionList})
	var data SessionListData
	if err := json.Unmarshal(list.Data, &data); err != nil {
		t.Fatalf("decoding list: %v", err)
	}
	if len(data.Sessions) != 1 || data.Sessions[0].Session != id || data.Sessions[0].Mode != "values" {
		t.Errorf("session_list = %+v, want one session %q in values mode", data.Sessions, id)
	}

	if resp := service.Handle(Request{Command: CommandSessionRm, Session: id}); !resp.OK {
		t.Fatalf("session_rm failed: %s", resp.Error)
	}

	empty := service.Handle(Request{Command: CommandSessionList})
	_ = json.Unmarshal(empty.Data, &data)
	if len(data.Sessions) != 0 {
		t.Errorf("session_list after rm = %+v, want empty", data.Sessions)
	}
}

func TestHandleServerVersion(t *testing.T) {
	service := NewService("v1.2.3", false)
	resp := service.Handle(Request{Command: CommandServerVersion})
	if !resp.OK || resp.Output != "v1.2.3\n" {
		t.Errorf("server_version = (ok %v, output %q), want \"v1.2.3\\n\"", resp.OK, resp.Output)
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	service := NewService("dev", false)
	resp := service.Handle(Request{Command: "bogus"})
	if resp.OK || !strings.Contains(resp.Error, "unknown command") {
		t.Errorf("unknown command = (ok %v, error %q), want failure", resp.OK, resp.Error)
	}
}
