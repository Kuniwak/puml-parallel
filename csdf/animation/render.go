// Package animation drives a Composable State Diagram (CSDF) as an interactive
// exploration: it steps through state groups by binding state-variable values,
// selects outgoing transitions, branches through history, and exposes the trace
// of the current path. It is UI- and transport-agnostic so a terminal REPL, a
// Unix-socket daemon, or a future web server can all drive the same engine, and
// it provides the canonical textual rendering of an exploration step.
package animation

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// HistoryEntry is one explored state together with the event trace that led to
// it from the initial state.
type HistoryEntry struct {
	State State    `json:"state"`
	Trace []string `json:"trace"`
}

// RenderError writes a human-readable error block.
func RenderError(w io.Writer, message string) {
	_, _ = fmt.Fprintf(w, "Error: %s\n\n", message)
}

// RenderEmptyLine writes a single blank line.
func RenderEmptyLine(w io.Writer) {
	_, _ = fmt.Fprintln(w)
}

// RenderStateValuePrompt renders the prompt shown before the user enters the
// values of the pending target state of diagram: the previous state, the
// guard and the post as the canonical clause printer writes them, and the
// variables to enter, with what an instance of a family's map holds.
func RenderStateValuePrompt(w io.Writer, diagram *core.Diagram, pending Pending) {
	if pending.Prev == nil {
		_, _ = fmt.Fprintln(w, "State: (none)")
	} else {
		_, _ = fmt.Fprintf(w, "State: %s\n", pending.Prev.Name)
		renderStateValues(w, pending.Prev.Values, "  ")
	}
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, "Guard:")
	_, _ = fmt.Fprintf(w, "  %s\n\n", ClauseText(pending.Guard))

	_, _ = fmt.Fprintln(w, "Post State Group:")
	_, _ = fmt.Fprintf(w, "  %s\n", pending.Target.Name)
	for _, name := range pending.Target.Vars {
		f, ok := family(diagram, name)
		if !ok {
			_, _ = fmt.Fprintf(w, "    %s'\n", name)
			continue
		}
		_, _ = fmt.Fprintf(w, "    %s', a map from IDs to instances {\"at\": <state>, \"fields\": {...}}:\n", name)
		for _, s := range f.States {
			if s == f.Init {
				continue
			}
			holding := "nothing"
			if vs := f.Vars[s]; len(vs) > 0 {
				holding = strings.Join(vs, ", ")
			}
			_, _ = fmt.Fprintf(w, "      at %s, holding %s\n", s, holding)
		}
	}
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, "Post Condition:")
	_, _ = fmt.Fprintf(w, "  %s\n\n", ClauseText(pending.Post))
	_, _ = fmt.Fprintln(w, valuePromptInstruction(pending.Target.Vars))
	_, _ = fmt.Fprintln(w)
}

// valuePromptInstruction tells the user exactly how many values to enter and in
// what order, using the variable names as placeholders.
func valuePromptInstruction(vars []string) string {
	if len(vars) == 0 {
		return "Enter an empty JSON array: []."
	}
	placeholders := make([]string, len(vars))
	for i, v := range vars {
		placeholders[i] = "<" + v + ">"
	}
	noun := "value"
	if len(vars) != 1 {
		noun = "values"
	}
	return fmt.Sprintf("Enter %d %s as a JSON array in declaration order: [%s].", len(vars), noun, strings.Join(placeholders, ", "))
}

// RenderState renders the current state, its values, its outgoing
// transitions and its end edges (each numbered for selection), or a
// deadlock notice if it has neither, as csdfbmc counts a deadlock.
func RenderState(w io.Writer, diagram *core.Diagram, state State) {
	edges := Outgoing(diagram, state.ID)
	ends := EndsFrom(diagram, state.ID)
	_, _ = fmt.Fprintf(w, "State: %s\n", state.Name)
	if len(edges) > 0 || len(ends) > 0 {
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintln(w, "Values:")
	if len(state.Values) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	}
	renderStateValues(w, state.Values, "  ")
	_, _ = fmt.Fprintln(w)

	if len(edges) == 0 && len(ends) == 0 {
		_, _ = fmt.Fprintln(w, "Deadlock: no outgoing transitions.")
		_, _ = fmt.Fprintln(w)
		return
	}
	if len(edges) > 0 {
		_, _ = fmt.Fprintln(w, "Transitions:")
	}
	for i, edge := range edges {
		_, _ = fmt.Fprintf(w, "  [%d] %s -> %s\n", i, edge.Event.Label(), StateName(diagram, edge.Dst))
		_, _ = fmt.Fprintf(w, "      Guard: %s\n", ClauseText(edge.Guard))
		_, _ = fmt.Fprintf(w, "      Post: %s\n", ClauseText(edge.Post))
		_, _ = fmt.Fprintln(w)
	}
	if len(ends) > 0 {
		_, _ = fmt.Fprintln(w, "Ends:")
	}
	for i, end := range ends {
		_, _ = fmt.Fprintf(w, "  [%d] end the run\n", i)
		_, _ = fmt.Fprintf(w, "      Guard: %s\n", ClauseText(end.Guard))
		_, _ = fmt.Fprintln(w)
	}
}

// RenderEnded renders the notice of a run that ended at state by an end
// edge.
func RenderEnded(w io.Writer, state State) {
	_, _ = fmt.Fprintf(w, "Ended: the run terminates at %s. Jump to a history entry (j INDEX) to go on.\n\n", state.Name)
}

func renderStateValues(w io.Writer, values []Value, indent string) {
	for _, value := range values {
		_, _ = fmt.Fprintf(w, "%s%s = %s\n", indent, value.Name, encodeJSON(value.Value))
	}
}

func encodeJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// ClauseText is c as the canonical clause printer writes it (clause.Print),
// or "(none)" for a clause of no conjunct, which holds.
func ClauseText(c clause.And) string {
	if len(c.Conjuncts) == 0 {
		return "(none)"
	}
	text, err := clause.Print(c)
	if err != nil {
		return fmt.Sprintf("(unprintable: %v)", err)
	}
	return text
}

// RenderTrace renders the event trace of the current path.
func RenderTrace(w io.Writer, trace []string) {
	encoded, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		RenderError(w, fmt.Sprintf("encoding trace: %v", err))
		return
	}
	indented := strings.ReplaceAll(string(encoded), "\n", "\n  ")
	_, _ = fmt.Fprintf(w, "Trace:\n  %s\n\n", indented)
}

// RenderHistory renders every explored history entry with its trace and values.
func RenderHistory(w io.Writer, history []HistoryEntry) {
	_, _ = fmt.Fprintln(w, "History:")
	for i, entry := range history {
		_, _ = fmt.Fprintf(w, "  [%d] Trace:\n", i)
		for _, event := range entry.Trace {
			_, _ = fmt.Fprintf(w, "        %s\n", event)
		}
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "      State:")
		_, _ = fmt.Fprintf(w, "        %s\n", entry.State.Name)
		renderStateValues(w, entry.State.Values, "          ")
		_, _ = fmt.Fprintln(w)
	}
}

// RenderHelp renders the REPL command reference.
func RenderHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  l         list the current state and transitions")
	_, _ = fmt.Fprintln(w, "  t         display the current trace")
	_, _ = fmt.Fprintln(w, "  h         display history")
	_, _ = fmt.Fprintln(w, "  s INDEX   select a transition")
	_, _ = fmt.Fprintln(w, "  e INDEX   take an end edge: the run terminates")
	_, _ = fmt.Fprintln(w, "  j INDEX   jump to a history entry")
	_, _ = fmt.Fprintln(w, "  ?, help   display this help")
	_, _ = fmt.Fprintln(w)
}
