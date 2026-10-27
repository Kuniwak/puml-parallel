package proto

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

const twoStateDiagram = `@startuml
state "Initial" as s0
s0 : count ; count
state "Done" as s1
s1 : result ; result
[*] --> s0 : $count' = 0
s0 --> s1 : insert?coin ; @ne(count, 9) ∧ $count >= 0 ; $result' is done
note as channels
  channel insert : coin
end note
@enduml
`

func intPtr(i int) *int { return &i }

// newSession creates a service holding one session at the initial value prompt
// and returns the service together with the new session id.
func newSession(t *testing.T) (*Service, string) {
	t.Helper()
	return newSessionChecking(t, AnyEnvironment)
}

// newSessionChecking is newSession with a service whose replays check the
// report's environment with check.
func newSessionChecking(t *testing.T, check EnvironmentCheck) (*Service, string) {
	t.Helper()
	service := NewService("test-version", false, check, animation.NoSuggesters)
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
	service := NewService("dev", false, AnyEnvironment, animation.NoSuggesters)
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
	service := NewService("dev", false, AnyEnvironment, animation.NoSuggesters)
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
	if !strings.Contains(resp.Output, "State: Initial") || !strings.Contains(resp.Output, "Transitions:") {
		t.Errorf("statevar output = %q, want state + transitions", resp.Output)
	}
	view := decodeView(t, resp)
	if view.Mode != "command" || view.State == nil || view.State.ID != "s0" {
		t.Errorf("statevar view = %+v, want command mode at s0", view)
	}
	if len(view.Transitions) != 1 || view.Transitions[0].Event != "insert?coin" {
		t.Errorf("statevar transitions = %+v, want one insert?coin", view.Transitions)
	}
}

func TestHandleStatevarReportsSolverErrors(t *testing.T) {
	tests := []struct {
		name      string
		values    string
		wantError string
	}{
		{name: "syntax", values: "null", wantError: "top-level value must be an array"},
		{name: "not JSON", values: "[", wantError: "invalid JSON array"},
		{name: "length", values: "[]", wantError: "expected 1 value(s) for [count], got 0"},
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

func TestHandleSessionNewErrorDebugSurfacesChain(t *testing.T) {
	oldSyntax := "@startuml\nstate \"S\" as s\n[*] --> s\ns --> s : a ; @opaque\n@enduml\n"
	tests := map[string]struct {
		debug    bool
		wantPart string
		wantLeak bool
	}{
		"default: the deepest message": {debug: false, wantPart: "@opaque", wantLeak: false},
		"debug: the full chain":        {debug: true, wantPart: "animation.ReadDiagram: coreio.ReadExpandedBytes:", wantLeak: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			service := NewService("dev", tt.debug, AnyEnvironment, animation.NoSuggesters)

			// Execute
			resp := service.Handle(Request{Command: CommandSessionNew, Path: "d.puml", Content: []byte(oldSyntax)})

			// Assert
			if resp.OK || !strings.Contains(resp.Error, tt.wantPart) {
				t.Fatalf("session_new = %+v, want an error containing %q", resp, tt.wantPart)
			}
			if leaks := strings.Contains(resp.Error, "animation.ReadDiagram"); leaks != tt.wantLeak {
				t.Errorf("session_new error %q: shows the chain %t, want %t", resp.Error, leaks, tt.wantLeak)
			}
		})
	}
}

func TestHandleSelectShowsTheClausesCanonically(t *testing.T) {
	// Setup
	service, id := advance(t)

	// Execute
	resp := service.Handle(Request{Command: CommandRead, Session: id})

	// Assert
	want := []Transition{{Index: 0, Event: "insert?coin", Dst: "s1", DstName: "Done",
		Guard: "@ne(count, 9) ∧ $count >= 0", Post: "$result' is done"}}
	if diff := cmp.Diff(want, decodeView(t, resp).Transitions); diff != "" {
		t.Errorf("transitions mismatch (-want +got):\n%s", diff)
	}
}

func TestAnEmptyClauseIsNoneInTheViewAsInTheText(t *testing.T) {
	// Setup
	service, id := newSession(t)

	// Execute
	resp := service.Handle(Request{Command: CommandRead, Session: id})

	// Assert
	view := decodeView(t, resp)
	if view.Pending == nil || view.Pending.Guard != "(none)" {
		t.Errorf("pending = %+v, want the start edge's empty guard as (none)", view.Pending)
	}
	if !strings.Contains(resp.Output, "Guard:\n  (none)\n") {
		t.Errorf("output = %q, want the empty guard as (none)", resp.Output)
	}
}

// promotionGlobal promotes a family of items, whose local diagram it
// includes as item.puml.
const promotionGlobal = `@startuml G
state "Run" as run {
  run : items ; ID ⇸ Item
  state "items : ID ⇸ Item" as runItems <<promote>> {
    !include item.puml
  }
}
[*] --> run
@enduml
`

// promotionLocal is the item: its start post says nothing, so promotion
// drops it with a warning.
const promotionLocal = `@startuml item
state "none" as itNone
state "made" as itMade
itMade : n ; n
[*] --> itNone : 準備済み
itNone --> itMade : MAKE ; ; $n' は 0
@enduml
`

func TestHandleSessionNewExploresTheExpansionOfAPromotion(t *testing.T) {
	// Setup
	service := NewService("dev", false, AnyEnvironment, animation.NoSuggesters)

	// Execute
	resp := service.Handle(Request{Command: CommandSessionNew, Path: "g.puml", Content: []byte(promotionGlobal),
		Includes: map[string]string{"item.puml": promotionLocal}})
	taken := service.Handle(Request{Command: CommandStatevar, Session: resp.Session, Values: `[{}]`})

	// Assert
	if !resp.OK {
		t.Fatalf("session_new failed: %s", resp.Error)
	}
	if len(resp.Warnings) == 0 {
		t.Errorf("session_new warnings = %v, want the dropped start post", resp.Warnings)
	}
	if !taken.OK || !strings.Contains(taken.Output, "MAKE?") {
		t.Errorf("statevar = %+v, want the session at the expansion's mode", taken)
	}
}

func TestHandleSessionNewRefusesAnIncludeTheRequestDoesNotCarry(t *testing.T) {
	// Setup
	service := NewService("dev", false, AnyEnvironment, animation.NoSuggesters)

	// Execute
	resp := service.Handle(Request{Command: CommandSessionNew, Path: "g.puml", Content: []byte(promotionGlobal)})

	// Assert
	if resp.OK || !strings.Contains(resp.Error, "item.puml") {
		t.Errorf("session_new = %+v, want the missing include refused", resp)
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
	if len(data.Trace) != 1 || data.Trace[0] != "insert?coin" {
		t.Errorf("trace = %v, want [insert?coin]", data.Trace)
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
	wantText := "ID\tMODE\tSTATE\tPATH\n" + id + "\tvalues\tInitial\tdiagram.puml\n"
	if list.Output != wantText {
		t.Errorf("session_list text = %q, want header + row %q", list.Output, wantText)
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
	service := NewService("v1.2.3", false, AnyEnvironment, animation.NoSuggesters)
	resp := service.Handle(Request{Command: CommandServerVersion})
	if !resp.OK || resp.Output != "v1.2.3\n" {
		t.Errorf("server_version = (ok %v, output %q), want \"v1.2.3\\n\"", resp.OK, resp.Output)
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	service := NewService("dev", false, AnyEnvironment, animation.NoSuggesters)
	resp := service.Handle(Request{Command: "bogus"})
	if resp.OK || !strings.Contains(resp.Error, "unknown command") {
		t.Errorf("unknown command = (ok %v, error %q), want failure", resp.OK, resp.Error)
	}
}

func TestHandleReplayWalksAFreshSessionToTheFinding(t *testing.T) {
	service, id := newSession(t)
	replay := animation.Replay{Initial: []byte(`[0]`), Steps: []animation.ReplayStep{{Event: "insert?coin", Src: "s0", Dst: "s1", Line: 7, Values: []byte(`["ok"]`)}}}
	resp := service.Handle(Request{Command: CommandReplay, Session: id, Replay: &replay})
	if !resp.OK {
		t.Fatalf("replay failed: %s", resp.Error)
	}
	view := decodeView(t, resp)
	if view.Mode != "command" || view.State == nil || view.State.ID != "s1" {
		t.Errorf("replay view = %+v, want command mode at s1", view)
	}
	hist := service.Handle(Request{Command: CommandHistory, Session: id})
	if !strings.Contains(hist.Output, "insert?coin") {
		t.Errorf("history after replay = %q, want the replayed event", hist.Output)
	}
}

func TestHandleReplayReportsAStepThatCannotBeTaken(t *testing.T) {
	service, id := newSession(t)
	replay := animation.Replay{Initial: []byte(`[0]`), Steps: []animation.ReplayStep{{Event: "eject", Src: "s0", Dst: "s1", Values: []byte(`[]`)}}}
	resp := service.Handle(Request{Command: CommandReplay, Session: id, Replay: &replay})
	if resp.OK || !strings.Contains(resp.Error, "eject") {
		t.Errorf("replay = %+v, want an error naming the step", resp)
	}
	read := service.Handle(Request{Command: CommandRead, Session: id})
	if view := decodeView(t, read); view.Mode != "command" || view.State == nil || view.State.ID != "s0" {
		t.Errorf("after a failed replay the session is %+v, want command mode at s0", view)
	}
}

type fakeSuggester struct {
	animation.NoSuggester
	values string
}

func (f fakeSuggester) Initial() ([]animation.Suggestion, error) {
	return []animation.Suggestion{{Values: []byte(f.values)}}, nil
}
func (f fakeSuggester) Next(animation.State, int) ([]animation.Suggestion, error) {
	return []animation.Suggestion{{Values: []byte(`["picked"]`)}}, nil
}

func TestSuggestionsShowAtThePromptAndCanBePicked(t *testing.T) {
	cleaned := false
	service := NewService("test-version", false, AnyEnvironment, func(*core.Diagram) (animation.Suggester, func(), error) {
		return fakeSuggester{values: `[0]`}, func() { cleaned = true }, nil
	})
	resp := service.Handle(Request{Command: CommandSessionNew, Path: "diagram.puml", Content: []byte(twoStateDiagram)})
	if !resp.OK {
		t.Fatalf("session_new failed: %s", resp.Error)
	}
	id := resp.Session

	read := service.Handle(Request{Command: CommandRead, Session: id})
	if !strings.Contains(read.Output, "[#0] [0]") {
		t.Errorf("read output = %q, want the suggestion", read.Output)
	}
	if view := decodeView(t, read); view.Pending == nil || len(view.Pending.Suggestions) != 1 {
		t.Errorf("read view = %+v, want one suggestion", view)
	}
	if resp := service.Handle(Request{Command: CommandStatevar, Session: id, Pick: intPtr(0)}); !resp.OK {
		t.Fatalf("statevar -pick 0 failed: %s", resp.Error)
	}
	service.Handle(Request{Command: CommandSelect, Session: id, Index: intPtr(0)})
	resp = service.Handle(Request{Command: CommandStatevar, Session: id, Pick: intPtr(0)})
	if !resp.OK || !strings.Contains(resp.Output, `result = "picked"`) {
		t.Errorf("statevar -pick after select = %+v, want the picked value entered", resp)
	}
	if resp := service.Handle(Request{Command: CommandStatevar, Session: id, Pick: intPtr(4)}); resp.OK {
		t.Error("picking a suggestion that does not exist succeeded")
	}
	service.Handle(Request{Command: CommandSessionRm, Session: id})
	if !cleaned {
		t.Error("removing the session did not run the suggester's cleanup")
	}
}

func TestPickWithoutACheckerIsRefused(t *testing.T) {
	service, id := newSession(t)
	resp := service.Handle(Request{Command: CommandStatevar, Session: id, Pick: intPtr(0)})
	if resp.OK || !strings.Contains(resp.Error, "-checks") {
		t.Errorf("statevar -pick without a checker = %+v, want a refusal naming -checks", resp)
	}
}

func TestHandleReplayRefusesAnEnvironmentTheDaemonDoesNotWalk(t *testing.T) {
	tests := map[string]struct {
		check   EnvironmentCheck
		wantErr string
	}{
		"any environment": {check: AnyEnvironment},
		"a check refusing": {
			check:   func(env *animation.Environment) error { return fmt.Errorf("%v refused", env.Env) },
			wantErr: "map[f:true] refused",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			service, id := newSessionChecking(t, tt.check)
			replay := animation.Replay{Initial: []byte(`[0]`)}

			// Execute
			resp := service.Handle(Request{Command: CommandReplay, Session: id, Replay: &replay, Environment: &animation.Environment{Given: []string{"f"}, Env: map[string]bool{"f": true}}})

			// Assert
			if tt.wantErr == "" {
				if !resp.OK {
					t.Fatalf("replay failed: %s", resp.Error)
				}
				return
			}
			if resp.OK || !strings.Contains(resp.Error, tt.wantErr) {
				t.Fatalf("replay = %+v, want an error saying %q", resp, tt.wantErr)
			}
			read := service.Handle(Request{Command: CommandRead, Session: id})
			if view := decodeView(t, read); view.Mode != "values" {
				t.Errorf("after a refused replay the session is %+v, want it untouched", view)
			}
		})
	}
}

const endingDiagram = "@startuml\nstate \"Idle\" as s0\ns0 : a ; a\n[*] --> s0 : $a' == 0\ns0 --> [*] : @eq(a, a)\n@enduml\n"

func TestHandleEndEndsTheRunAtAnEndEdge(t *testing.T) {
	// Setup
	service := NewService("dev", false, AnyEnvironment, animation.NoSuggesters)
	resp := service.Handle(Request{Command: CommandSessionNew, Path: "e.puml", Content: []byte(endingDiagram)})
	if !resp.OK {
		t.Fatalf("Setup: session_new failed: %s", resp.Error)
	}
	id := resp.Session
	read := service.Handle(Request{Command: CommandStatevar, Session: id, Values: "[0]"})
	if !read.OK {
		t.Fatalf("Setup: statevar failed: %s", read.Error)
	}

	// Execute
	ended := service.Handle(Request{Command: CommandEnd, Session: id, Index: intPtr(0)})
	again := service.Handle(Request{Command: CommandEnd, Session: id, Index: intPtr(0)})

	// Assert
	if strings.Contains(read.Output, "Deadlock") || !strings.Contains(read.Output, "Ends:\n  [0] end the run\n      Guard: @eq(a, a)\n") {
		t.Errorf("read output = %q, want the end edge and no deadlock", read.Output)
	}
	if diff := cmp.Diff([]End{{Index: 0, Guard: "@eq(a, a)"}}, decodeView(t, read).Ends); diff != "" {
		t.Errorf("ends mismatch (-want +got):\n%s", diff)
	}
	if !ended.OK || decodeView(t, ended).Mode != "ended" || !strings.Contains(ended.Output, "Ended: the run terminates at Idle.") {
		t.Errorf("end = %+v, want the run ended", ended)
	}
	if again.OK || !strings.Contains(again.Error, "the run has ended") {
		t.Errorf("end after the end = %+v, want a refusal", again)
	}
}
