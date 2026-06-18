package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

const tau core.Event = "tau"

type StateValue struct {
	Name  core.Var `json:"name"`
	Value any      `json:"value"`
}

type RuntimeState struct {
	ID     core.StateID `json:"state_id"`
	Name   string       `json:"state_name"`
	Values []StateValue `json:"values"`
}

type HistoryEntry struct {
	State RuntimeState `json:"state"`
	Trace []core.Event `json:"trace"`
}

type PostSolverResultKind int

const (
	PostSolverResultOK PostSolverResultKind = iota
	PostSolverResultNoSolutions
	PostSolverResultInvalidStateVarValuesLength
	PostSolverResultSyntaxError
)

type PostSolverInput struct {
	StateGroup    core.State
	Previous      *RuntimeState
	Guard         string
	Post          string
	EncodedValues string
}

type PostSolverResult struct {
	Kind  PostSolverResultKind
	State RuntimeState
	Err   error
}

type PostSolver interface {
	Solve(PostSolverInput) PostSolverResult
}

type JSONPostSolver struct{}

func (JSONPostSolver) Solve(input PostSolverInput) PostSolverResult {
	decoder := json.NewDecoder(strings.NewReader(input.EncodedValues))
	decoder.UseNumber()

	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: fmt.Errorf("invalid JSON array: %w", err)}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: err}
	}
	values, ok := decoded.([]any)
	if !ok {
		return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: errors.New("invalid JSON array: top-level value must be an array")}
	}
	for _, value := range values {
		if containsNull(value) {
			return PostSolverResult{Kind: PostSolverResultSyntaxError, Err: errors.New("null is not a supported JSON value")}
		}
	}
	if len(values) != len(input.StateGroup.Vars) {
		return PostSolverResult{Kind: PostSolverResultInvalidStateVarValuesLength}
	}

	stateValues := make([]StateValue, len(values))
	for i, value := range values {
		stateValues[i] = StateValue{Name: input.StateGroup.Vars[i].Name, Value: value}
	}
	return PostSolverResult{
		Kind: PostSolverResultOK,
		State: RuntimeState{
			ID:     input.StateGroup.ID,
			Name:   input.StateGroup.Name,
			Values: stateValues,
		},
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("invalid JSON array: %w", err)
	}
	return errors.New("invalid JSON array: multiple JSON values")
}

func containsNull(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case []any:
		for _, item := range value {
			if containsNull(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range value {
			if containsNull(item) {
				return true
			}
		}
	}
	return false
}

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s <file.puml|file.png>\n", os.Args[0])
		_, _ = fmt.Fprintln(os.Stderr, "Interactively explores a Composable State Diagram.")
	}
	flag.Parse()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	if exitCode := Run(flag.Args(), os.Stdin, os.Stdout, os.Stderr, interrupts); exitCode != 0 {
		os.Exit(exitCode)
	}
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, interrupts <-chan os.Signal) int {
	return runWithSolver(args, stdin, stdout, stderr, interrupts, JSONPostSolver{})
}

func runWithSolver(args []string, stdin io.Reader, stdout, stderr io.Writer, interrupts <-chan os.Signal, solver PostSolver) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "Error: exactly one CSDF file is required")
		return 1
	}

	diagram, err := loadDiagram(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	lines := newLineReader(stdin)
	repl := repl{
		diagram:    diagram,
		stdout:     stdout,
		interrupts: interrupts,
		lines:      lines,
		solver:     solver,
	}
	return repl.run()
}

func loadDiagram(path string) (*core.Diagram, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("reading PlantUML source from %s: %w", path, err)
	}
	diagram, err := core.NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", path, err)
	}
	return diagram, nil
}

type lineResult struct {
	line string
	err  error
}

func newLineReader(reader io.Reader) <-chan lineResult {
	results := make(chan lineResult)
	go func() {
		defer close(results)
		buffered := bufio.NewReader(reader)
		for {
			line, err := buffered.ReadString('\n')
			if err == nil {
				line = strings.TrimSuffix(line, "\n")
				line = strings.TrimSuffix(line, "\r")
				results <- lineResult{line: line}
			}
			if err != nil {
				results <- lineResult{err: err}
				return
			}
		}
	}()
	return results
}

type repl struct {
	diagram    *core.Diagram
	stdout     io.Writer
	interrupts <-chan os.Signal
	lines      <-chan lineResult
	solver     PostSolver
	history    []HistoryEntry
}

func (r *repl) run() int {
	initial, ok := r.diagram.States[r.diagram.StartEdge.Dst]
	if !ok {
		r.displayFatal(fmt.Sprintf("initial state %q does not exist", r.diagram.StartEdge.Dst))
		return 1
	}

	var previous *RuntimeState
	stateGroup := initial
	guard := core.True
	post := r.diagram.StartEdge.Post
	event := tau

	for {
		state, outcome := r.askStateValues(stateGroup, previous, guard, post, event)
		switch outcome {
		case inputExit:
			return 0
		case inputFatal:
			return 1
		case inputBack:
			current := r.history[len(r.history)-1].State
			previous = &current
		default:
			current := state
			previous = &current
		}

		r.displayState(*previous)
		for {
			current := *previous
			command, outcome := r.readLine("command> ")
			if outcome == inputExit || outcome == inputInterrupt {
				return 0
			}
			if outcome == inputFatal {
				return 1
			}

			action, index, err := parseCommand(command)
			if err != nil {
				r.displayError(err.Error())
				continue
			}

			switch action {
			case commandEmpty:
				r.displayEmptyLine()
			case commandList:
				r.displayState(current)
			case commandTrace:
				r.displayJSON("Trace", r.currentTrace())
			case commandHistory:
				r.displayJSON("History", r.history)
			case commandHelp:
				r.displayHelp()
			case commandJump:
				if index >= len(r.history) {
					r.displayError("Index out of range")
					continue
				}
				entry := cloneHistoryEntry(r.history[index])
				r.history = append(r.history, entry)
				jumped := entry.State
				previous = &jumped
				r.displayState(jumped)
			case commandSelect:
				edges := r.outgoing(current.ID)
				if index >= len(edges) {
					r.displayError("Index out of range")
					continue
				}
				edge := edges[index]
				next, ok := r.diagram.States[edge.Dst]
				if !ok {
					r.displayFatal(fmt.Sprintf("destination state %q does not exist", edge.Dst))
					return 1
				}
				stateGroup = next
				guard = edge.Guard
				post = edge.Post
				event = edge.Event
				goto askValues
			}
		}

	askValues:
	}
}

type inputOutcome int

const (
	inputLine inputOutcome = iota
	inputExit
	inputInterrupt
	inputBack
	inputFatal
)

func (r *repl) askStateValues(group core.State, previous *RuntimeState, guard, post string, event core.Event) (RuntimeState, inputOutcome) {
	for {
		r.displayStateValuePrompt(group, guard, post)
		line, outcome := r.readLine("state> ")
		if outcome == inputExit {
			return RuntimeState{}, inputExit
		}
		if outcome == inputFatal {
			return RuntimeState{}, inputFatal
		}
		if outcome == inputInterrupt {
			if len(r.history) == 0 {
				r.displayFatal("No solutions found")
				return RuntimeState{}, inputFatal
			}
			return RuntimeState{}, inputBack
		}

		result := r.solver.Solve(PostSolverInput{
			StateGroup:    group,
			Previous:      previous,
			Guard:         guard,
			Post:          post,
			EncodedValues: line,
		})
		switch result.Kind {
		case PostSolverResultOK:
			trace := append([]core.Event{}, r.currentTrace()...)
			if event != tau {
				trace = append(trace, event)
			}
			r.history = append(r.history, HistoryEntry{State: result.State, Trace: trace})
			return result.State, inputLine
		case PostSolverResultNoSolutions:
			r.displayError("No solutions")
		case PostSolverResultInvalidStateVarValuesLength:
			r.displayError("State variable values length mismatch")
		case PostSolverResultSyntaxError:
			if result.Err == nil {
				r.displayError("invalid state variable values")
			} else {
				r.displayError(result.Err.Error())
			}
		default:
			r.displayFatal("post solver returned an unknown result")
			return RuntimeState{}, inputFatal
		}
	}
}

func (r *repl) readLine(prompt string) (string, inputOutcome) {
	_, _ = fmt.Fprint(r.stdout, prompt)
	select {
	case <-r.interrupts:
		_, _ = fmt.Fprintln(r.stdout)
		return "", inputInterrupt
	default:
	}
	select {
	case <-r.interrupts:
		_, _ = fmt.Fprintln(r.stdout)
		return "", inputInterrupt
	case result, ok := <-r.lines:
		if !ok || errors.Is(result.err, io.EOF) {
			return "", inputExit
		}
		if result.err != nil {
			r.displayFatal(fmt.Sprintf("reading input: %v", result.err))
			return "", inputFatal
		}
		return result.line, inputLine
	}
}

func (r *repl) displayFatal(message string) {
	_, _ = fmt.Fprintf(r.stdout, "Fatal: %s\n", message)
}

func (r *repl) displayError(message string) {
	_, _ = fmt.Fprintf(r.stdout, "Error: %s\n", message)
}

func (r *repl) displayEmptyLine() {
	_, _ = fmt.Fprintln(r.stdout)
}

func (r *repl) displayStateValuePrompt(group core.State, guard, post string) {
	_, _ = fmt.Fprintf(r.stdout, "State: %s (%s)\n", group.Name, group.ID)
	_, _ = fmt.Fprintln(r.stdout, "Variables:")
	if len(group.Vars) == 0 {
		_, _ = fmt.Fprintln(r.stdout, "  (none)")
	}
	for _, variable := range group.Vars {
		if variable.Type == "" {
			_, _ = fmt.Fprintf(r.stdout, "  %s\n", variable.Name)
		} else {
			_, _ = fmt.Fprintf(r.stdout, "  %s: %s\n", variable.Name, variable.Type)
		}
	}
	_, _ = fmt.Fprintf(r.stdout, "Guard: %s\n", displayCondition(guard))
	_, _ = fmt.Fprintf(r.stdout, "Post: %s\n", displayCondition(post))
	_, _ = fmt.Fprintln(r.stdout, "Enter state variable values as a JSON array.")
}

func (r *repl) displayState(state RuntimeState) {
	_, _ = fmt.Fprintf(r.stdout, "State: %s (%s)\n", state.Name, state.ID)
	_, _ = fmt.Fprintln(r.stdout, "Values:")
	if len(state.Values) == 0 {
		_, _ = fmt.Fprintln(r.stdout, "  (none)")
	}
	for _, value := range state.Values {
		encoded, _ := json.Marshal(value.Value)
		_, _ = fmt.Fprintf(r.stdout, "  %s = %s\n", value.Name, encoded)
	}

	edges := r.outgoing(state.ID)
	if len(edges) == 0 {
		_, _ = fmt.Fprintln(r.stdout, "Deadlock: no outgoing transitions.")
		return
	}
	_, _ = fmt.Fprintln(r.stdout, "Transitions:")
	for i, edge := range edges {
		destination := r.diagram.States[edge.Dst]
		_, _ = fmt.Fprintf(r.stdout, "  [%d] %s -> %s (%s)\n", i, edge.Event, destination.Name, edge.Dst)
		_, _ = fmt.Fprintf(r.stdout, "      Guard: %s\n", displayCondition(edge.Guard))
		_, _ = fmt.Fprintf(r.stdout, "      Post: %s\n", displayCondition(edge.Post))
	}
}

func displayCondition(condition string) string {
	if condition == "" {
		return core.True
	}
	return condition
}

func (r *repl) displayJSON(label string, value any) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		r.displayError(fmt.Sprintf("encoding %s: %v", strings.ToLower(label), err))
		return
	}
	_, _ = fmt.Fprintf(r.stdout, "%s:\n%s", label, output.String())
}

func (r *repl) displayHelp() {
	_, _ = fmt.Fprintln(r.stdout, "Commands:")
	_, _ = fmt.Fprintln(r.stdout, "  l         list the current state and transitions")
	_, _ = fmt.Fprintln(r.stdout, "  t         display the current trace")
	_, _ = fmt.Fprintln(r.stdout, "  h         display history")
	_, _ = fmt.Fprintln(r.stdout, "  s INDEX   select a transition")
	_, _ = fmt.Fprintln(r.stdout, "  j INDEX   jump to a history entry")
	_, _ = fmt.Fprintln(r.stdout, "  ?, help   display this help")
}

func (r *repl) outgoing(stateID core.StateID) []core.Edge {
	var edges []core.Edge
	for _, edge := range r.diagram.Edges {
		if edge.Src == stateID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func (r *repl) currentTrace() []core.Event {
	if len(r.history) == 0 {
		return nil
	}
	return r.history[len(r.history)-1].Trace
}

func cloneHistoryEntry(entry HistoryEntry) HistoryEntry {
	entry.State.Values = append([]StateValue{}, entry.State.Values...)
	entry.Trace = append([]core.Event{}, entry.Trace...)
	return entry
}

type commandKind int

const (
	commandEmpty commandKind = iota
	commandList
	commandTrace
	commandHistory
	commandHelp
	commandSelect
	commandJump
)

func parseCommand(input string) (commandKind, int, error) {
	switch input {
	case "":
		return commandEmpty, 0, nil
	case "l":
		return commandList, 0, nil
	case "t":
		return commandTrace, 0, nil
	case "h":
		return commandHistory, 0, nil
	case "?", "help":
		return commandHelp, 0, nil
	case "s":
		return 0, 0, errors.New("required an index of transition")
	case "j":
		return 0, 0, errors.New("required an index of history")
	}

	for _, command := range []string{"l", "t", "h", "?", "help"} {
		if strings.HasPrefix(input, command+" ") {
			return 0, 0, errors.New("invalid arguments")
		}
	}
	if strings.HasPrefix(input, "s ") {
		index, err := parseNaturalNumber(strings.TrimPrefix(input, "s "))
		return commandSelect, index, err
	}
	if strings.HasPrefix(input, "j ") {
		index, err := parseNaturalNumber(strings.TrimPrefix(input, "j "))
		return commandJump, index, err
	}
	return 0, 0, errors.New("invalid command")
}

func parseNaturalNumber(input string) (int, error) {
	if input == "" {
		return 0, errors.New("Not a natural number")
	}
	index, err := strconv.Atoi(input)
	if err != nil || index < 0 {
		return 0, errors.New("Not a natural number")
	}
	return index, nil
}
