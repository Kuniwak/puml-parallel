package animation

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const counterSource = `@startuml
state "counting" as a
a: n
state "stopped" as b
[*] --> a : $n' = 2
a --> a : dec ; $n > 0 ; $n' = $n - 1
a --> b : stop ; $n = 0
@enduml
`

func TestReplayWalksTheSessionAlongTheSteps(t *testing.T) {
	sess := newSession(t, counterSource)
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
	sess := newSession(t, counterSource)
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
[*] --> s : $n' = 0
s --> s : tick ; $n = 0 ; $n' = 1
s --> s : tick ; $n = 0 ; $n' = 2
@enduml
`
	sess := newSession(t, source)
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

// A state where csdfbmc reached the bound of a sample type is no finding, but
// it can be replayed; it comes after the findings, so their numbers stay.
func TestLoadReplaysListsTheBoundReachedLast(t *testing.T) {
	// Setup
	bs := []byte(`{"deadlocks": [{"at": {"state": "b", "values": []}, "trace": [], "replay": {"initial": [], "steps": []}}],
	  "tau_cycles": [], "violations": [], "avoiding_cycles": [], "empty_posts": [], "unconstrained": [],
	  "bound_reached": [{"at": {"state": "r", "values": [{}]}, "trace": ["OPEN(k=a)"],
	  "replay": {"initial": [{}], "steps": [{"event": "OPEN?k", "src": "r", "dst": "r", "line": 3, "values": [{}]}]}}]}`)

	// Execute
	findings, err := LoadReplays(bs)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || findings[0].Kind != "deadlock" || findings[1].Kind != "bound_reached" || len(findings[1].Replay.Steps) != 1 {
		t.Errorf("findings = %+v, want the deadlock, then the bound reached with its replay", findings)
	}
}

func TestReplayRefusesAValueTheConeLeftOut(t *testing.T) {
	sess := newSession(t, counterSource)
	err := sess.Replay(Replay{Initial: []byte(`[2]`), Steps: []ReplayStep{{Event: "dec", Src: "a", Dst: "a", Line: 6, Values: []byte(`[{"left_out": true}]`)}}})
	if err == nil || !strings.Contains(err.Error(), "without --cone") {
		t.Fatalf("Replay() error = %v, want one suggesting a run without --cone", err)
	}
	if len(sess.History()) != 0 {
		t.Errorf("history has %d entries, want none: nothing is replayed", len(sess.History()))
	}
}

func TestLoadEnvironmentReadsWhatReproducesTheRun(t *testing.T) {
	tests := map[string]struct {
		report string
		want   *Environment
	}{
		"given functions": {
			report: `{"environment": {"given": ["$_0 が 正", "f"], "env": {"f": false}, "decided": ["f"]}}`,
			want:   &Environment{Given: []string{"$_0 が 正", "f"}, Env: map[string]bool{"f": false}},
		},
		"a seed, which is not read": {
			report: `{"environment": {"seed": 7, "given": [], "env": {}, "decided": []}}`,
			want:   &Environment{Given: []string{}, Env: map[string]bool{}},
		},
		"nothing given": {
			report: `{"environment": {"given": [], "env": {}, "decided": []}}`,
			want:   &Environment{Given: []string{}, Env: map[string]bool{}},
		},
		"a report of no environment": {
			report: `{"deadlocks": []}`,
			want:   nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := LoadEnvironment([]byte(tt.report))

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("LoadEnvironment() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadEnvironmentRefusesWhatIsNotAReport(t *testing.T) {
	// Execute
	_, err := LoadEnvironment([]byte("not json"))

	// Assert
	if err == nil {
		t.Error("LoadEnvironment() = nil, want an error")
	}
}

func TestLoadFindingPicksOneFindingAndTheEnvironment(t *testing.T) {
	two := `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}},{"replay":{"initial":[1],"steps":[]}}],"environment":{"given":[],"env":{},"decided":[]}}`
	tests := map[string]struct {
		report      string
		n           int
		wantInitial string
		wantEnv     *Environment
		wantErr     string
	}{
		"the first":          {report: two, n: 0, wantInitial: "[0]", wantEnv: &Environment{Given: []string{}, Env: map[string]bool{}}},
		"the second":         {report: two, n: 1, wantInitial: "[1]", wantEnv: &Environment{Given: []string{}, Env: map[string]bool{}}},
		"no environment":     {report: `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}]}`, wantInitial: "[0]"},
		"past the last":      {report: two, n: 2, wantErr: "no finding 2; the report records 2 (0 to 1)"},
		"a negative index":   {report: two, n: -1, wantErr: "no finding -1"},
		"no finding":         {report: `{"deadlocks":[]}`, wantErr: "the report records no finding"},
		"not csdfbmc's JSON": {report: "not json", wantErr: "not csdfbmc --json output"},
		"an environment of no env": {
			report:  `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}],"environment":{"given":["f"],"decided":[]}}`,
			wantErr: "records no env; make it again with this release's csdfbmc --json",
		},
		"an env of a function not given": {
			report:  `{"deadlocks":[{"replay":{"initial":[0],"steps":[]}}],"environment":{"given":[],"env":{"f":true},"decided":[]}}`,
			wantErr: `gives "f" a value, but does not list it as given`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			finding, env, err := LoadFinding([]byte(tt.report), tt.n)

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadFinding() error = %v, want one saying %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(finding.Replay.Initial) != tt.wantInitial {
				t.Errorf("LoadFinding() initial = %s, want %s", finding.Replay.Initial, tt.wantInitial)
			}
			if diff := cmp.Diff(tt.wantEnv, env); diff != "" {
				t.Errorf("LoadFinding() environment mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
