package animation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Replay is a path through a diagram as csdfbmc records it under a finding:
// the values of the initial state and, per step, the edge taken and the
// values of the state it leads to, each as the JSON array SolveJSON reads.
type Replay struct {
	Initial json.RawMessage `json:"initial"`
	Steps   []ReplayStep    `json:"steps"`
}

// ReplayStep names an edge by its event text, source, destination and source
// line (0 when the edge has none, as in an expansion), which together pick
// it out among the outgoing transitions, and carries the values entered after it.
type ReplayStep struct {
	Event  csdf.Event      `json:"event"`
	Src    csdf.StateID    `json:"src"`
	Dst    csdf.StateID    `json:"dst"`
	Line   int             `json:"line"`
	Values json.RawMessage `json:"values"`
}

// Finding is one entry of csdfbmc's JSON output with its replay: Kind is the
// list it came from (deadlock, tau_cycle, violation, avoiding_cycle,
// empty_post, unconstrained), Detail what that list says about it.
type Finding struct {
	Kind   string
	Detail string
	Trace  []string
	Replay Replay
}

// Replay walks a fresh session along r: the initial values, then every step.
// It stops at the first step that does not match an outgoing transition or
// whose values the solver refuses, leaving the session at the last state it
// reached, in command mode.
func (s *Session) Replay(r Replay) error {
	if s.Mode() != ModeValues || len(s.history) != 0 {
		return errors.New("animation.Session.Replay: a replay starts from a fresh session")
	}
	if err := r.checkNothingLeftOut(); err != nil {
		return err
	}
	if err := s.enterReplayValues(r.Initial, "the initial state"); err != nil {
		return err
	}
	for i, step := range r.Steps {
		idx, err := s.matchStep(step)
		if err != nil {
			return fmt.Errorf("animation.Session.Replay: step %d: %w", i+1, err)
		}
		if err := s.Select(idx); err != nil {
			return fmt.Errorf("animation.Session.Replay: step %d: %w", i+1, err)
		}
		if err := s.enterReplayValues(step.Values, fmt.Sprintf("step %d (%s)", i+1, step.Event)); err != nil {
			_ = s.Back()
			return err
		}
	}
	return nil
}

// checkNothingLeftOut refuses a replay holding a value csdfbmc --cone left
// out, {"left_out": true}: no value of the variable stands for it, so the
// session could not enter it.
func (r Replay) checkNothingLeftOut() error {
	values := []json.RawMessage{r.Initial}
	for _, step := range r.Steps {
		values = append(values, step.Values)
	}
	for i, raw := range values {
		if len(raw) == 0 {
			continue
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("animation.Session.Replay: the values %s are no JSON: %w", raw, err)
		}
		if leftOut(v) {
			what := "the initial state"
			if i > 0 {
				what = fmt.Sprintf("step %d (%s)", i, r.Steps[i-1].Event)
			}
			return fmt.Errorf("animation.Session.Replay: %s holds a value csdfbmc --cone left out, which a replay cannot enter; "+
				"run csdfbmc without --cone to replay this finding", what)
		}
	}
	return nil
}

func leftOut(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if b, ok := x["left_out"].(bool); ok && b && len(x) == 1 {
			return true
		}
		for _, y := range x {
			if leftOut(y) {
				return true
			}
		}
	case []any:
		for _, y := range x {
			if leftOut(y) {
				return true
			}
		}
	}
	return false
}

func (s *Session) enterReplayValues(values json.RawMessage, what string) error {
	encoded := string(values)
	if strings.TrimSpace(encoded) == "" {
		encoded = "[]"
	}
	result, err := s.EnterValues(encoded)
	if err != nil {
		return fmt.Errorf("animation.Session.Replay: %s: %w", what, err)
	}
	if result.Kind != csdf.PostSolverResultOK {
		if result.Err != nil {
			return fmt.Errorf("animation.Session.Replay: %s: the values %s were refused: %w", what, encoded, result.Err)
		}
		return fmt.Errorf("animation.Session.Replay: %s: the values %s were refused", what, encoded)
	}
	return nil
}

// matchStep finds the outgoing transition a step names. The event text and
// the destination narrow it; the line settles parallel edges when it is known.
func (s *Session) matchStep(step ReplayStep) (int, error) {
	edges := s.Transitions()
	var candidates []int
	for i, e := range edges {
		if e.Event == step.Event && e.Dst == step.Dst && (step.Src == "" || e.Src == step.Src) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) > 1 && step.Line != 0 {
		var byLine []int
		for _, i := range candidates {
			if edges[i].Line == step.Line {
				byLine = append(byLine, i)
			}
		}
		if len(byLine) > 0 {
			candidates = byLine
		}
	}
	switch len(candidates) {
	case 0:
		return 0, fmt.Errorf("no transition %q to %q from the current state", step.Event, step.Dst)
	case 1:
		return candidates[0], nil
	default:
		return 0, fmt.Errorf("transition %q to %q is ambiguous and no line settles it", step.Event, step.Dst)
	}
}

// LoadReplays reads csdfbmc's JSON output and flattens its findings, in the
// order the output lists them: deadlocks, tau cycles, broken invariants,
// avoiding cycles, empty posts, unconstrained posts; then the states where
// the bound of a sample type was reached, which are no findings but may be
// looked at too.
func LoadReplays(bs []byte) ([]Finding, error) {
	type entry struct {
		At        json.RawMessage `json:"at"`
		Trace     []string        `json:"trace"`
		Replay    Replay          `json:"replay"`
		Invariant string          `json:"invariant"`
		Event     string          `json:"event"`
		Variables []string        `json:"variables"`
	}
	var out struct {
		Deadlocks      []entry `json:"deadlocks"`
		TauCycles      []entry `json:"tau_cycles"`
		Violations     []entry `json:"violations"`
		AvoidingCycles []entry `json:"avoiding_cycles"`
		EmptyPosts     []entry `json:"empty_posts"`
		Unconstrained  []entry `json:"unconstrained"`
		BoundReached   []entry `json:"bound_reached"`
	}
	if err := json.Unmarshal(bs, &out); err != nil {
		return nil, fmt.Errorf("animation.LoadReplays: not csdfbmc --json output: %w", err)
	}
	var findings []Finding
	add := func(kind string, entries []entry, detail func(entry) string) {
		for _, e := range entries {
			findings = append(findings, Finding{Kind: kind, Detail: detail(e), Trace: e.Trace, Replay: e.Replay})
		}
	}
	none := func(entry) string { return "" }
	add("deadlock", out.Deadlocks, none)
	add("tau_cycle", out.TauCycles, none)
	add("violation", out.Violations, func(e entry) string { return e.Invariant })
	add("avoiding_cycle", out.AvoidingCycles, none)
	add("empty_post", out.EmptyPosts, func(e entry) string { return e.Event })
	add("unconstrained", out.Unconstrained, func(e entry) string { return e.Event + " leaves " + strings.Join(e.Variables, ", ") })
	add("bound_reached", out.BoundReached, none)
	return findings, nil
}
