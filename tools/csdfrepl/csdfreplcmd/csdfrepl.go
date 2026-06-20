package csdfreplcmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
	"golang.org/x/term"
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

func runWithSolver(file string, inout *cli.ProcInout, interrupts <-chan os.Signal, solver PostSolver) error {
	diagram, err := loadDiagram(file)
	if err != nil {
		return err
	}

	var terminal *terminalLineReader
	var lines <-chan lineResult
	stdinFile, stdinIsFile := inout.Stdin.(*os.File)
	stdoutFile, stdoutIsFile := inout.Stdout.(*os.File)
	if stdinIsFile && stdoutIsFile && term.IsTerminal(int(stdinFile.Fd())) && term.IsTerminal(int(stdoutFile.Fd())) {
		terminal = newTerminalLineReader(stdinFile, stdoutFile)
	} else {
		lines = newLineReader(inout.Stdin)
	}
	repl := repl{
		diagram:    diagram,
		stdout:     inout.Stdout,
		interrupts: interrupts,
		lines:      lines,
		terminal:   terminal,
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

var errTerminalInterrupt = errors.New("terminal input interrupted")

type terminalReadWriter struct {
	reader *bufio.Reader
	writer io.Writer
}

func (rw *terminalReadWriter) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	value, err := rw.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if value == 3 {
		return 0, errTerminalInterrupt
	}
	buffer[0] = value
	return 1, nil
}

func (rw *terminalReadWriter) Write(buffer []byte) (int, error) {
	return rw.writer.Write(buffer)
}

type terminalLineReader struct {
	stream   *terminalReadWriter
	inputFD  int
	outputFD int
}

func newTerminalLineReader(reader io.Reader, writer io.Writer) *terminalLineReader {
	inputFD := -1
	if file, ok := reader.(*os.File); ok {
		inputFD = int(file.Fd())
	}
	outputFD := -1
	if file, ok := writer.(*os.File); ok {
		outputFD = int(file.Fd())
	}
	return &terminalLineReader{
		stream: &terminalReadWriter{
			reader: bufio.NewReader(reader),
			writer: writer,
		},
		inputFD:  inputFD,
		outputFD: outputFD,
	}
}

func (r *terminalLineReader) readLine(prompt string) (string, error) {
	if r.inputFD >= 0 {
		oldState, err := term.MakeRaw(r.inputFD)
		if err != nil {
			return "", fmt.Errorf("configuring terminal input: %w", err)
		}
		defer func() {
			_ = term.Restore(r.inputFD, oldState)
		}()
	}
	terminal := term.NewTerminal(r.stream, prompt)
	if r.outputFD >= 0 {
		if width, height, err := term.GetSize(r.outputFD); err == nil {
			_ = terminal.SetSize(width, height)
		}
	}
	return terminal.ReadLine()
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
	terminal   *terminalLineReader
	solver     PostSolver
	history    []HistoryEntry
}

func (r *repl) run() error {
	initial, ok := r.diagram.States[r.diagram.StartEdge.Dst]
	if !ok {
		return fmt.Errorf("initial state %q does not exist", r.diagram.StartEdge.Dst)
	}

	var previous *RuntimeState
	stateGroup := initial
	guard := core.True
	post := r.diagram.StartEdge.Post
	event := tau

	for {
		state, outcome, err := r.askStateValues(stateGroup, previous, guard, post, event)
		if err != nil {
			return err
		}
		switch outcome {
		case inputExit:
			return nil
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
			command, outcome, err := r.readLine("command> ")
			if err != nil {
				return err
			}
			if outcome == inputExit || outcome == inputInterrupt {
				return nil
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
				r.displayTrace(r.currentTrace())
			case commandHistory:
				r.displayHistory(r.history)
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
					return fmt.Errorf("destination state %q does not exist", edge.Dst)
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

func (r *repl) askStateValues(group core.State, previous *RuntimeState, guard, post string, event core.Event) (RuntimeState, inputOutcome, error) {
	for {
		r.displayStateValuePrompt(previous, group, guard, post)
		line, outcome, err := r.readLine("state> ")
		if err != nil {
			return RuntimeState{}, inputFatal, err
		}
		if outcome == inputExit {
			return RuntimeState{}, inputExit, nil
		}
		if outcome == inputInterrupt {
			if len(r.history) == 0 {
				return RuntimeState{}, inputFatal, errors.New("No solutions found")
			}
			return RuntimeState{}, inputBack, nil
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
			return result.State, inputLine, nil
		case PostSolverResultNoSolutions:
			r.displayStateVarsError("No solutions")
		case PostSolverResultInvalidStateVarValuesLength:
			r.displayStateVarsError("State variable values length mismatch")
		case PostSolverResultSyntaxError:
			if result.Err == nil {
				r.displayStateVarsError("invalid state variable values")
			} else {
				r.displayStateVarsError(result.Err.Error())
			}
		default:
			return RuntimeState{}, inputFatal, errors.New("post solver returned an unknown result")
		}
	}
}

func (r *repl) readLine(prompt string) (string, inputOutcome, error) {
	if r.terminal != nil {
		select {
		case <-r.interrupts:
			_, _ = fmt.Fprintln(r.stdout, prompt)
			return "", inputInterrupt, nil
		default:
		}
		line, err := r.terminal.readLine(prompt)
		if errors.Is(err, errTerminalInterrupt) {
			_, _ = fmt.Fprintln(r.stdout)
			return "", inputInterrupt, nil
		}
		if errors.Is(err, io.EOF) {
			return "", inputExit, nil
		}
		if err != nil {
			return "", inputFatal, fmt.Errorf("reading input: %w", err)
		}
		return line, inputLine, nil
	}

	_, _ = fmt.Fprint(r.stdout, prompt)
	select {
	case <-r.interrupts:
		_, _ = fmt.Fprintln(r.stdout)
		return "", inputInterrupt, nil
	default:
	}
	select {
	case <-r.interrupts:
		_, _ = fmt.Fprintln(r.stdout)
		return "", inputInterrupt, nil
	case result, ok := <-r.lines:
		if !ok || errors.Is(result.err, io.EOF) {
			return "", inputExit, nil
		}
		if result.err != nil {
			return "", inputFatal, fmt.Errorf("reading input: %w", result.err)
		}
		_, _ = fmt.Fprintln(r.stdout)
		return result.line, inputLine, nil
	}
}

func (r *repl) displayError(message string) {
	_, _ = fmt.Fprintf(r.stdout, "Error: %s\n", message)
}

func (r *repl) displayStateVarsError(message string) {
	r.displayError(message)
	_, _ = fmt.Fprintln(r.stdout)
}

func (r *repl) displayEmptyLine() {
	_, _ = fmt.Fprintln(r.stdout)
}

func (r *repl) displayStateValuePrompt(previous *RuntimeState, group core.State, guard, post string) {
	if previous == nil {
		_, _ = fmt.Fprintln(r.stdout, "State: (none)")
	} else {
		_, _ = fmt.Fprintf(r.stdout, "State: %s (%s)\n", previous.Name, previous.ID)
		r.displayStateValues(previous.Values, "  ")
	}
	_, _ = fmt.Fprintln(r.stdout)

	_, _ = fmt.Fprintln(r.stdout, "Guard:")
	_, _ = fmt.Fprintf(r.stdout, "  %s\n\n", displayCondition(guard))

	_, _ = fmt.Fprintln(r.stdout, "Post State Group:")
	_, _ = fmt.Fprintf(r.stdout, "  %s\n", group.Name)
	for _, variable := range group.Vars {
		variableType := variable.Type
		if variableType == "" {
			variableType = "any"
		}
		_, _ = fmt.Fprintf(r.stdout, "    %s' as %s\n", variable.Name, variableType)
	}
	_, _ = fmt.Fprintln(r.stdout)

	_, _ = fmt.Fprintln(r.stdout, "Post Condition:")
	_, _ = fmt.Fprintf(r.stdout, "  %s\n\n", displayCondition(post))
	_, _ = fmt.Fprintln(r.stdout, "Enter state variable values as a JSON array.")
	_, _ = fmt.Fprintln(r.stdout)
}

func (r *repl) displayState(state RuntimeState) {
	edges := r.outgoing(state.ID)
	_, _ = fmt.Fprintf(r.stdout, "State: %s (%s)\n", state.Name, state.ID)
	if len(edges) > 0 {
		_, _ = fmt.Fprintln(r.stdout)
	}
	_, _ = fmt.Fprintln(r.stdout, "Values:")
	if len(state.Values) == 0 {
		_, _ = fmt.Fprintln(r.stdout, "  (none)")
	}
	r.displayStateValues(state.Values, "  ")
	_, _ = fmt.Fprintln(r.stdout)

	if len(edges) == 0 {
		_, _ = fmt.Fprintln(r.stdout, "Deadlock: no outgoing transitions.")
		_, _ = fmt.Fprintln(r.stdout)
		return
	}
	_, _ = fmt.Fprintln(r.stdout, "Transitions:")
	for i, edge := range edges {
		destination := r.diagram.States[edge.Dst]
		_, _ = fmt.Fprintf(r.stdout, "  [%d] %s -> %s (%s)\n", i, edge.Event, destination.Name, edge.Dst)
		_, _ = fmt.Fprintf(r.stdout, "      Guard: %s\n", displayCondition(edge.Guard))
		_, _ = fmt.Fprintf(r.stdout, "      Post: %s\n", displayCondition(edge.Post))
		_, _ = fmt.Fprintln(r.stdout)
	}
}

func (r *repl) displayStateValues(values []StateValue, indent string) {
	for _, value := range values {
		_, _ = fmt.Fprintf(r.stdout, "%s%s = %s\n", indent, value.Name, encodeJSON(value.Value))
	}
}

func encodeJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func displayCondition(condition string) string {
	if condition == "" {
		return core.True
	}
	return condition
}

func (r *repl) displayTrace(trace []core.Event) {
	encoded, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		r.displayError(fmt.Sprintf("encoding trace: %v", err))
		return
	}
	indented := strings.ReplaceAll(string(encoded), "\n", "\n  ")
	_, _ = fmt.Fprintf(r.stdout, "Trace:\n  %s\n\n", indented)
}

func (r *repl) displayHistory(history []HistoryEntry) {
	_, _ = fmt.Fprintln(r.stdout, "History:")
	for i, entry := range history {
		_, _ = fmt.Fprintf(r.stdout, "  [%d] Trace:\n", i)
		for _, event := range entry.Trace {
			_, _ = fmt.Fprintf(r.stdout, "        %s\n", event)
		}
		_, _ = fmt.Fprintln(r.stdout)
		_, _ = fmt.Fprintln(r.stdout, "      State:")
		_, _ = fmt.Fprintf(r.stdout, "        %s\n", entry.State.Name)
		r.displayStateValues(entry.State.Values, "          ")
		_, _ = fmt.Fprintln(r.stdout)
	}
}

func (r *repl) displayHelp() {
	_, _ = fmt.Fprintln(r.stdout, "Commands:")
	_, _ = fmt.Fprintln(r.stdout, "  l         list the current state and transitions")
	_, _ = fmt.Fprintln(r.stdout, "  t         display the current trace")
	_, _ = fmt.Fprintln(r.stdout, "  h         display history")
	_, _ = fmt.Fprintln(r.stdout, "  s INDEX   select a transition")
	_, _ = fmt.Fprintln(r.stdout, "  j INDEX   jump to a history entry")
	_, _ = fmt.Fprintln(r.stdout, "  ?, help   display this help")
	_, _ = fmt.Fprintln(r.stdout)
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
