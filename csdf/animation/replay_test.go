package animation

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
)

const counterSource = `@startuml
state "counting" as a
a: n
state "stopped" as b
[*] --> a : n = 2
a --> a : dec ; n > 0 ; n' = n - 1
a --> b : stop ; n = 0
@enduml
`

func TestReplayWalksTheSessionAlongTheSteps(t *testing.T) {
	sess, err := NewSession(csdf.MustParse(counterSource), csdf.SolveJSON)
	if err != nil {
		t.Fatal(err)
	}
	replay := Replay{
		Initial: []byte(`[2]`),
		Steps: []ReplayStep{
			{Event: "dec", Src: "a", Dst: "a", Line: 6, Values: []byte(`[1]`)},
			{Event: "dec", Src: "a", Dst: "a", Line: 6, Values: []byte(`[0]`)},
			{Event: "stop", Src: "a", Dst: "b", Line: 7, Values: []byte(`[]`)},
		},
	}
	if err := sess.Replay(replay); err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	current, ok := sess.Current()
	if !ok || current.ID != "b" {
		t.Errorf("after the replay the session is at %v (command mode %t), want b", current, ok)
	}
	if got := sess.Trace(); len(got) != 3 || got[2] != "stop" {
		t.Errorf("trace = %v, want dec dec stop", got)
	}
	if len(sess.History()) != 4 {
		t.Errorf("history has %d entries, want 4 (initial and three steps)", len(sess.History()))
	}
}

func TestReplayRefusesAStepThatMatchesNoTransition(t *testing.T) {
	sess, _ := NewSession(csdf.MustParse(counterSource), csdf.SolveJSON)
	err := sess.Replay(Replay{Initial: []byte(`[2]`), Steps: []ReplayStep{{Event: "jump", Src: "a", Dst: "b", Values: []byte(`[]`)}}})
	if err == nil || !strings.Contains(err.Error(), "jump") {
		t.Fatalf("Replay() error = %v, want one naming the event", err)
	}
	// The session stays usable at the last state it reached.
	if current, ok := sess.Current(); !ok || current.ID != "a" {
		t.Errorf("after a failed step the session is at %v, want a", current)
	}
}

func TestReplayMatchesAnEdgeByLineWhenEventsRepeat(t *testing.T) {
	source := `@startuml
state "s" as s
s: n
[*] --> s : n = 0
s --> s : tick ; n = 0 ; n' = 1
s --> s : tick ; n = 0 ; n' = 2
@enduml
`
	sess, _ := NewSession(csdf.MustParse(source), csdf.SolveJSON)
	err := sess.Replay(Replay{Initial: []byte(`[0]`), Steps: []ReplayStep{{Event: "tick", Src: "s", Dst: "s", Line: 6, Values: []byte(`[2]`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := sess.History()[1].State.Values[0].Value; got.(interface{ String() string }).String() != "2" {
		t.Errorf("value = %v, want 2", got)
	}
}

// csdfbmc --json lists its findings by kind; LoadReplays flattens them into
// one list a user can pick from by index.
func TestLoadReplaysFlattensTheFindings(t *testing.T) {
	bs := []byte(`{"states": 4, "deadlocks": [{"at": {"state": "b", "values": []}, "trace": ["dec","dec","stop"],
	  "replay": {"initial": [2], "steps": [{"event": "dec", "src": "a", "dst": "a", "line": 6, "values": [1]}]}}],
	  "tau_cycles": [], "violations": [{"invariant": "n ≥ 1", "at": {"state": "a", "values": [0]}, "trace": [],
	  "replay": {"initial": [0], "steps": []}}], "avoiding_cycles": [], "empty_posts": [], "unconstrained": []}`)
	findings, err := LoadReplays(bs)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}
	if findings[0].Kind != "deadlock" || findings[1].Kind != "violation" || findings[1].Detail != "n ≥ 1" {
		t.Errorf("findings = %+v", findings)
	}
	if string(findings[0].Replay.Initial) != "[2]" || len(findings[0].Replay.Steps) != 1 || findings[0].Replay.Steps[0].Line != 6 {
		t.Errorf("replay = %+v", findings[0].Replay)
	}
}
