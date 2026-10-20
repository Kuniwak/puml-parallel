package animation

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Suggestion is one set of values a checker says the pending post admits:
// the parameters of the event it assumed, and the values as the JSON array
// SolveJSON reads.
type Suggestion struct {
	Params map[string]any  `json:"params"`
	Values json.RawMessage `json:"values"`
}

// Suggester answers what a value prompt may ask a checker: the values the
// start state admits, and the values after the idx-th outgoing transition of
// a state. An error is the checker's, and a REPL shows it and goes on
// without suggestions.
type Suggester interface {
	Initial() ([]Suggestion, error)
	Next(from csdf.RuntimeState, idx int) ([]Suggestion, error)
}

// Suggest asks the suggester about the session's pending transition: the
// start state when nothing has been explored yet, otherwise the transition
// last selected. It returns nil, nil when there is nothing pending.
func (s *Session) Suggest(sg Suggester) ([]Suggestion, error) {
	if s.mode != ModeValues {
		return nil, nil
	}
	if s.pending.prev == nil {
		return sg.Initial()
	}
	edges := Outgoing(s.diagram, s.pending.prev.ID)
	for i, e := range edges {
		if e.Event == s.pending.event && e.Dst == s.pending.group.ID && e.Guard == s.pending.guard && e.Post == s.pending.post {
			return sg.Next(*s.pending.prev, i)
		}
	}
	return nil, fmt.Errorf("animation.Session.Suggest: the pending transition is not among the outgoing ones")
}

// RenderSuggestions renders what the checker admits, numbered so that a
// user can enter "#N" instead of the values.
func RenderSuggestions(w io.Writer, suggestions []Suggestion) {
	if len(suggestions) == 0 {
		_, _ = fmt.Fprintln(w, "Suggested by csdfbmc: nothing; no values satisfy this transition under the twins.")
		_, _ = fmt.Fprintln(w)
		return
	}
	_, _ = fmt.Fprintln(w, "Suggested by csdfbmc (enter #N to take one):")
	for i, s := range suggestions {
		line := fmt.Sprintf("  [#%d] %s", i, strings.TrimSpace(string(s.Values)))
		if len(s.Params) > 0 {
			params, _ := json.Marshal(s.Params)
			line += "  with " + string(params)
		}
		_, _ = fmt.Fprintln(w, line)
	}
	_, _ = fmt.Fprintln(w)
}

// PickSuggestion resolves an input of the form "#N" against suggestions; ok
// is false when the input is not of that form.
func PickSuggestion(input string, suggestions []Suggestion) (values string, ok bool, err error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "#") {
		return "", false, nil
	}
	var n int
	if _, err := fmt.Sscanf(input[1:], "%d", &n); err != nil {
		return "", true, fmt.Errorf("enter #N with N the number of a suggestion")
	}
	if n < 0 || n >= len(suggestions) {
		return "", true, fmt.Errorf("no suggestion #%d; there are %d", n, len(suggestions))
	}
	return string(suggestions[n].Values), true, nil
}
