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
	"github.com/Kuniwak/puml-parallel/csdf"
	"golang.org/x/term"
)

const tau csdf.Event = "tau"

type HistoryEntry struct {
	State csdf.RuntimeState `json:"state"`
	Trace []csdf.Event      `json:"trace"`
}

func runWithSolver(file string, inout *cli.ProcInout, interrupts <-chan os.Signal, solver csdf.PostSolver) error {
	bs, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("csdfreplcmd.runWithSolver: cannot read the file: %w: %q", err, file)
	}

	diagram, err := csdf.ParseDiagram(bs)
	if err != nil {
		return fmt.Errorf("csdfreplcmd.runWithSolver: cannot parse the file: %w: %q", err, file)
	}

	var terminal *terminalLineReader
	var lines <-chan lineResult
	if term.IsTerminal(int(inout.Stdin.Fd())) && term.IsTerminal(int(inout.Stdout.Fd())) {
		terminal = newTerminalLineReader(inout)
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

func newTerminalLineReader(inout *cli.ProcInout) *terminalLineReader {
	return &terminalLineReader{
		stream: &terminalReadWriter{
			reader: bufio.NewReader(inout.Stdin),
			writer: inout.Stdout,
		},
		inputFD:  int(inout.Stdin.Fd()),
		outputFD: int(inout.Stdout.Fd()),
	}
}

func (r *terminalLineReader) readLine(prompt string) (string, error) {
	if term.IsTerminal(r.inputFD) {
		oldState, err := term.MakeRaw(r.inputFD)
		if err != nil {
			return "", fmt.Errorf("csdfreplcmd.terminalLineReader.readLine: configuring terminal input: %w", err)
		}
		defer func() {
			_ = term.Restore(r.inputFD, oldState)
		}()
	}
	terminal := term.NewTerminal(r.stream, prompt)
	if term.IsTerminal(r.outputFD) {
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
	diagram    *csdf.Diagram
	stdout     io.Writer
	interrupts <-chan os.Signal
	lines      <-chan lineResult
	terminal   *terminalLineReader
	solver     csdf.PostSolver
	history    []HistoryEntry
}

func (r *repl) run() error {
	initial, ok := r.diagram.States[r.diagram.StartEdge.Dst]
	if !ok {
		return fmt.Errorf("csdfreplcmd.repl.run: initial state %q does not exist", r.diagram.StartEdge.Dst)
	}

	var previous *csdf.RuntimeState
	stateGroup := initial
	guard := csdf.True
	post := r.diagram.StartEdge.Post
	event := tau

	for {
		state, outcome, err := r.askStateValues(stateGroup, previous, guard, post, event)
		if err != nil {
			return fmt.Errorf("csdfreplcmd.repl.run: %w", err)
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
				return fmt.Errorf("csdfreplcmd.repl.run: %w", err)
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
					return fmt.Errorf("csdfreplcmd.repl.run: destination state %q does not exist", edge.Dst)
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

func (r *repl) askStateValues(group csdf.State, previous *csdf.RuntimeState, guard, post string, event csdf.Event) (csdf.RuntimeState, inputOutcome, error) {
	for {
		r.displayStateValuePrompt(previous, group, guard, post)
		line, outcome, err := r.readLine("state> ")
		if err != nil {
			return csdf.RuntimeState{}, inputFatal, fmt.Errorf("csdfreplcmd.repl.askStateValues: %w", err)
		}
		if outcome == inputExit {
			return csdf.RuntimeState{}, inputExit, nil
		}
		if outcome == inputInterrupt {
			if len(r.history) == 0 {
				return csdf.RuntimeState{}, inputFatal, errors.New("csdfreplcmd.repl.askStateValues: No solutions found")
			}
			return csdf.RuntimeState{}, inputBack, nil
		}

		result := r.solver(csdf.PostSolverInput{
			StateGroup:    group,
			Previous:      previous,
			Guard:         guard,
			Post:          post,
			EncodedValues: line,
		})
		switch result.Kind {
		case csdf.PostSolverResultOK:
			trace := append([]csdf.Event{}, r.currentTrace()...)
			if event != tau {
				trace = append(trace, event)
			}
			r.history = append(r.history, HistoryEntry{State: result.State, Trace: trace})
			return result.State, inputLine, nil
		case csdf.PostSolverResultNoSolutions:
			r.displayError("No solutions")
		case csdf.PostSolverResultInvalidStateVarValuesLength:
			r.displayError("State variable values length mismatch")
		case csdf.PostSolverResultSyntaxError:
			if result.Err == nil {
				r.displayError("invalid state variable values")
			} else {
				r.displayError(result.Err.Error())
			}
		default:
			return csdf.RuntimeState{}, inputFatal, errors.New("csdfreplcmd.repl.askStateValues: post solver returned an unknown result")
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
			return "", inputFatal, fmt.Errorf("csdfreplcmd.repl.readLine: reading input: %w", err)
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
			return "", inputFatal, fmt.Errorf("csdfreplcmd.repl.readLine: reading input: %w", result.err)
		}
		_, _ = fmt.Fprintln(r.stdout)
		return result.line, inputLine, nil
	}
}

func (r *repl) displayError(message string) {
	_, _ = fmt.Fprintf(r.stdout, "Error: %s\n\n", message)
}

func (r *repl) displayEmptyLine() {
	_, _ = fmt.Fprintln(r.stdout)
}

func (r *repl) displayStateValuePrompt(previous *csdf.RuntimeState, group csdf.State, guard, post string) {
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

func (r *repl) displayState(state csdf.RuntimeState) {
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

func (r *repl) displayStateValues(values []csdf.StateValue, indent string) {
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
		return csdf.True
	}
	return condition
}

func (r *repl) displayTrace(trace []csdf.Event) {
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

func (r *repl) outgoing(stateID csdf.StateID) []csdf.Edge {
	var edges []csdf.Edge
	for _, edge := range r.diagram.Edges {
		if edge.Src == stateID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func (r *repl) currentTrace() []csdf.Event {
	if len(r.history) == 0 {
		return nil
	}
	return r.history[len(r.history)-1].Trace
}

func cloneHistoryEntry(entry HistoryEntry) HistoryEntry {
	entry.State.Values = append([]csdf.StateValue{}, entry.State.Values...)
	entry.Trace = append([]csdf.Event{}, entry.Trace...)
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
