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

	"github.com/Kuniwak/puml-parallel/csdf"
)

// HistoryEntry is one explored state together with the event trace that led to
// it from the initial state.
type HistoryEntry struct {
	State csdf.RuntimeState `json:"state"`
	Trace []csdf.Event      `json:"trace"`
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
// state-variable values for the pending post state group.
func RenderStateValuePrompt(w io.Writer, previous *csdf.RuntimeState, group csdf.State, guard, post string) {
	if previous == nil {
		_, _ = fmt.Fprintln(w, "State: (none)")
	} else {
		_, _ = fmt.Fprintf(w, "State: %s\n", previous.Name)
		renderStateValues(w, previous.Values, "  ")
	}
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, "Guard:")
	_, _ = fmt.Fprintf(w, "  %s\n\n", renderCondition(guard))

	_, _ = fmt.Fprintln(w, "Post State Group:")
	_, _ = fmt.Fprintf(w, "  %s\n", group.Name)
	for _, variable := range group.Vars {
		variableType := variable.Type
		if variableType == "" {
			variableType = "any"
		}
		_, _ = fmt.Fprintf(w, "    %s' as %s\n", variable.Name, variableType)
	}
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, "Post Condition:")
	_, _ = fmt.Fprintf(w, "  %s\n\n", renderCondition(post))
	_, _ = fmt.Fprintln(w, valuePromptInstruction(group.Vars))
	_, _ = fmt.Fprintln(w)
}

// valuePromptInstruction tells the user exactly how many values to enter and in
// what order, using the variable names as placeholders.
func valuePromptInstruction(vars []csdf.StateVar) string {
	if len(vars) == 0 {
		return "Enter an empty JSON array: []."
	}
	placeholders := make([]string, len(vars))
	for i, v := range vars {
		placeholders[i] = "<" + string(v.Name) + ">"
	}
	noun := "value"
	if len(vars) != 1 {
		noun = "values"
	}
	return fmt.Sprintf("Enter %d %s as a JSON array in declaration order: [%s].", len(vars), noun, strings.Join(placeholders, ", "))
}

// RenderState renders the current state, its values, and its outgoing
// transitions (numbered for selection), or a deadlock notice if there are none.
func RenderState(w io.Writer, diagram *csdf.Diagram, state csdf.RuntimeState) {
	edges := Outgoing(diagram, state.ID)
	_, _ = fmt.Fprintf(w, "State: %s\n", state.Name)
	if len(edges) > 0 {
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintln(w, "Values:")
	if len(state.Values) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	}
	renderStateValues(w, state.Values, "  ")
	_, _ = fmt.Fprintln(w)

	if len(edges) == 0 {
		_, _ = fmt.Fprintln(w, "Deadlock: no outgoing transitions.")
		_, _ = fmt.Fprintln(w)
		return
	}
	_, _ = fmt.Fprintln(w, "Transitions:")
	for i, edge := range edges {
		destination := diagram.States[edge.Dst]
		_, _ = fmt.Fprintf(w, "  [%d] %s -> %s\n", i, edge.Event, destination.Name)
		_, _ = fmt.Fprintf(w, "      Guard: %s\n", renderCondition(edge.Guard))
		_, _ = fmt.Fprintf(w, "      Post: %s\n", renderCondition(edge.Post))
		_, _ = fmt.Fprintln(w)
	}
}

func renderStateValues(w io.Writer, values []csdf.StateValue, indent string) {
	for _, value := range values {
		_, _ = fmt.Fprintf(w, "%s%s = %s\n", indent, value.Name, encodeJSON(value.Value))
	}
}

func encodeJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func renderCondition(condition string) string {
	if condition == "" {
		return csdf.True
	}
	return condition
}

// RenderTrace renders the event trace of the current path.
func RenderTrace(w io.Writer, trace []csdf.Event) {
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
	_, _ = fmt.Fprintln(w, "  j INDEX   jump to a history entry")
	_, _ = fmt.Fprintln(w, "  ?, help   display this help")
	_, _ = fmt.Fprintln(w)
}
