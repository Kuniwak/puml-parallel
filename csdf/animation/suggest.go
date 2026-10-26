package animation

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Suggestion is one set of values a checker says the pending post admits:
// the parameters of the event it assumed, and the values as the JSON array
// EnterValues reads.
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
	Next(from State, idx int) ([]Suggestion, error)
}

// Suggest asks the suggester about the session's pending transition: the
// start state when nothing has been explored yet, otherwise the transition
// last selected. It returns nil, nil when there is nothing pending.
func (s *Session) Suggest(sg Suggester) ([]Suggestion, error) {
	if s.mode != ModeValues {
		return nil, nil
	}
	if s.pending.Prev == nil {
		return sg.Initial()
	}
	return sg.Next(*s.pending.Prev, s.pending.edge)
}

// RenderSuggestions renders what the checker admits, numbered so that a
// user can enter "#N" instead of the values.
func RenderSuggestions(w io.Writer, suggestions []Suggestion) {
	if len(suggestions) == 0 {
		_, _ = fmt.Fprintln(w, "Suggested by csdfbmc: nothing; no values satisfy this transition.")
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
