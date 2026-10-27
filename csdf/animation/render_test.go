package animation

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRenderStateValuePrompt(t *testing.T) {
	tests := map[string]struct {
		source  string
		pending func(t *testing.T, s *Session) Pending
		want    string
	}{
		"the start state": {
			source:  twoStateDiagram,
			pending: func(t *testing.T, s *Session) Pending { return s.Pending() },
			want: "" +
				"State: (none)\n" +
				"\n" +
				"Guard:\n" +
				"  (none)\n" +
				"\n" +
				"Post State Group:\n" +
				"  Initial\n" +
				"    count'\n" +
				"\n" +
				"Post Condition:\n" +
				"  $count' = 0\n" +
				"\n" +
				"Enter 1 value as a JSON array in declaration order: [<count>].\n" +
				"\n",
		},
		"a transition of builtins, printed canonically": {
			source: builtinsDiagram,
			pending: func(t *testing.T, s *Session) Pending {
				if _, err := s.EnterValues(`[{}, 0]`); err != nil {
					t.Fatal(err)
				}
				if err := s.Select(0); err != nil {
					t.Fatal(err)
				}
				return s.Pending()
			},
			want: "" +
				"State: Open\n" +
				"  m = {}\n" +
				"  k = 0\n" +
				"\n" +
				"Guard:\n" +
				"  @notin(m, x) ∧ @ne(x, k)\n" +
				"\n" +
				"Post State Group:\n" +
				"  Open\n" +
				"    m'\n" +
				"    k'\n" +
				"\n" +
				"Post Condition:\n" +
				"  @update(m', m, x, 1) ∧ @eq(k', x)\n" +
				"\n" +
				"Enter 2 values as a JSON array in declaration order: [<m>, <k>].\n" +
				"\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := newSession(t, tt.source)
			pending := tt.pending(t, session)
			var buf bytes.Buffer

			// Execute
			RenderStateValuePrompt(&buf, session.Diagram(), pending)

			// Assert
			if diff := cmp.Diff(tt.want, buf.String()); diff != "" {
				t.Errorf("RenderStateValuePrompt() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderStateValuePromptShowsWhatAnInstanceOfAFamilyHolds(t *testing.T) {
	// Setup
	session, err := NewSession(promotedDiagram(), NoSuggester{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	// Execute
	RenderStateValuePrompt(&buf, session.Diagram(), session.Pending())

	// Assert
	want := "" +
		"State: (none)\n" +
		"\n" +
		"Guard:\n" +
		"  (none)\n" +
		"\n" +
		"Post State Group:\n" +
		"  G\n" +
		"    members', a map from IDs to instances {\"at\": <state>, \"fields\": {...}}:\n" +
		"      at opened, holding count\n" +
		"      at closed, holding nothing\n" +
		"    total'\n" +
		"\n" +
		"Post Condition:\n" +
		"  (none)\n" +
		"\n" +
		"Enter 2 values as a JSON array in declaration order: [<members>, <total>].\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("RenderStateValuePrompt() mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderState(t *testing.T) {
	tests := map[string]struct {
		source string
		values string
		want   string
	}{
		"a state with transitions": {
			source: twoStateDiagram,
			values: "[2]",
			want: "" +
				"State: Initial\n" +
				"\n" +
				"Values:\n" +
				"  count = 2\n" +
				"\n" +
				"Transitions:\n" +
				"  [0] insert?coin -> Done\n" +
				"      Guard: $count >= 0\n" +
				"      Post: $result' is done\n" +
				"\n",
		},
		"a state whose only exit is an end edge, which is no deadlock": {
			source: endingDiagram,
			values: "[0]",
			want: "" +
				"State: Idle\n" +
				"\n" +
				"Values:\n" +
				"  a = 0\n" +
				"\n" +
				"Ends:\n" +
				"  [0] end the run\n" +
				"      Guard: @eq(a, a)\n" +
				"\n",
		},
		"a state with a transition and an end edge": {
			source: "@startuml\nstate \"Idle\" as s0\n[*] --> s0\ns0 --> s0 : tick\ns0 --> [*]\n@enduml\n",
			values: "[]",
			want: "" +
				"State: Idle\n" +
				"\n" +
				"Values:\n" +
				"  (none)\n" +
				"\n" +
				"Transitions:\n" +
				"  [0] tick -> Idle\n" +
				"      Guard: (none)\n" +
				"      Post: (none)\n" +
				"\n" +
				"Ends:\n" +
				"  [0] end the run\n" +
				"      Guard: (none)\n" +
				"\n",
		},
		"a deadlock": {
			source: "@startuml\nstate \"End\" as e\n[*] --> e\n@enduml\n",
			values: "[]",
			want: "" +
				"State: End\n" +
				"Values:\n" +
				"  (none)\n" +
				"\n" +
				"Deadlock: no outgoing transitions.\n" +
				"\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session := newSession(t, tt.source)
			state, err := session.EnterValues(tt.values)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer

			// Execute
			RenderState(&buf, session.Diagram(), state)

			// Assert
			if diff := cmp.Diff(tt.want, buf.String()); diff != "" {
				t.Errorf("RenderState() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderEnded(t *testing.T) {
	// Setup
	var buf bytes.Buffer

	// Execute
	RenderEnded(&buf, State{ID: "s0", Name: "Idle"})

	// Assert
	want := "Ended: the run terminates at Idle. Jump to a history entry (j INDEX) to go on.\n\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}

func TestRenderTrace(t *testing.T) {
	// Setup
	var buf bytes.Buffer

	// Execute
	RenderTrace(&buf, []string{"submit?order", "approve"})

	// Assert
	want := "" +
		"Trace:\n" +
		"  [\n" +
		`    "submit?order",` + "\n" +
		`    "approve"` + "\n" +
		"  ]\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}

func TestRenderHelp(t *testing.T) {
	// Setup
	var buf bytes.Buffer

	// Execute
	RenderHelp(&buf)

	// Assert
	want := "" +
		"Commands:\n" +
		"  l         list the current state and transitions\n" +
		"  t         display the current trace\n" +
		"  h         display history\n" +
		"  s INDEX   select a transition\n" +
		"  e INDEX   take an end edge: the run terminates\n" +
		"  j INDEX   jump to a history entry\n" +
		"  ?, help   display this help\n" +
		"\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Error(diff)
	}
}
