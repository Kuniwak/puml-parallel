package csdfreplcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"golang.org/x/term"
)

// HistoryEntry is the explored-state record; the engine owns the type.
type HistoryEntry = animation.HistoryEntry

func runWithSolver(file string, inout *cli.ProcInout, interrupts <-chan os.Signal, solver csdf.PostSolver) error {
	bs, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("csdfreplcmd.runWithSolver: cannot read the file: %w: %q", err, file)
	}

	diagram, err := csdf.ParseDiagram(bs)
	if err != nil {
		return fmt.Errorf("csdfreplcmd.runWithSolver: cannot parse the file: %w: %q", err, file)
	}

	session, err := animation.NewSession(diagram, solver)
	if err != nil {
		return fmt.Errorf("csdfreplcmd.runWithSolver: %w", err)
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
		session:    session,
		stdout:     inout.Stdout,
		interrupts: interrupts,
		lines:      lines,
		terminal:   terminal,
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

// repl is the terminal front-end: it reads lines and renders, delegating all
// exploration state to the animation engine.
type repl struct {
	diagram    *csdf.Diagram
	session    *animation.Session
	stdout     io.Writer
	interrupts <-chan os.Signal
	lines      <-chan lineResult
	terminal   *terminalLineReader
}

func (r *repl) run() error {
	for {
		outcome, err := r.askStateValues()
		if err != nil {
			return fmt.Errorf("csdfreplcmd.repl.run: %w", err)
		}
		if outcome == inputExit {
			return nil
		}
		// On inputLine the session advanced to command mode; on inputBack the
		// session was returned to command mode at the last explored state.

		current, _ := r.session.Current()
		r.displayState(current)
		for {
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
				current, _ := r.session.Current()
				r.displayState(current)
			case commandTrace:
				r.displayTrace(r.session.Trace())
			case commandHistory:
				r.displayHistory(r.session.History())
			case commandHelp:
				r.displayHelp()
			case commandJump:
				if err := r.session.Jump(index); err != nil {
					if errors.Is(err, animation.ErrIndexOutOfRange) {
						r.displayError(err.Error())
						continue
					}
					return fmt.Errorf("csdfreplcmd.repl.run: %w", err)
				}
				current, _ := r.session.Current()
				r.displayState(current)
			case commandSelect:
				if err := r.session.Select(index); err != nil {
					if errors.Is(err, animation.ErrIndexOutOfRange) {
						r.displayError(err.Error())
						continue
					}
					return fmt.Errorf("csdfreplcmd.repl.run: %w", err)
				}
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

func (r *repl) askStateValues() (inputOutcome, error) {
	for {
		group, guard, post, _, prev := r.session.Pending()
		r.displayStateValuePrompt(prev, group, guard, post)
		line, outcome, err := r.readLine("state> ")
		if err != nil {
			return inputFatal, fmt.Errorf("csdfreplcmd.repl.askStateValues: %w", err)
		}
		if outcome == inputExit {
			return inputExit, nil
		}
		if outcome == inputInterrupt {
			if len(r.session.History()) == 0 {
				return inputFatal, errors.New("csdfreplcmd.repl.askStateValues: No solutions found")
			}
			if err := r.session.Back(); err != nil {
				return inputFatal, fmt.Errorf("csdfreplcmd.repl.askStateValues: %w", err)
			}
			return inputBack, nil
		}

		result, err := r.session.EnterValues(line)
		if err != nil {
			return inputFatal, fmt.Errorf("csdfreplcmd.repl.askStateValues: %w", err)
		}
		switch result.Kind {
		case csdf.PostSolverResultOK:
			return inputLine, nil
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
			return inputFatal, errors.New("csdfreplcmd.repl.askStateValues: post solver returned an unknown result")
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

// The display* methods bind the engine's renderers to the REPL's stdout and
// diagram so the run loop and the renderer tests share one rendering.
func (r *repl) displayError(message string) { animation.RenderError(r.stdout, message) }

func (r *repl) displayEmptyLine() { animation.RenderEmptyLine(r.stdout) }

func (r *repl) displayStateValuePrompt(previous *csdf.RuntimeState, group csdf.State, guard, post string) {
	animation.RenderStateValuePrompt(r.stdout, previous, group, guard, post)
}

func (r *repl) displayState(state csdf.RuntimeState) {
	animation.RenderState(r.stdout, r.diagram, state)
}

func (r *repl) displayTrace(trace []csdf.Event) { animation.RenderTrace(r.stdout, trace) }

func (r *repl) displayHistory(history []HistoryEntry) { animation.RenderHistory(r.stdout, history) }

func (r *repl) displayHelp() { animation.RenderHelp(r.stdout) }

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
