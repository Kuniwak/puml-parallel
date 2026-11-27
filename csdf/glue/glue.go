// Package glue speaks the evaluator protocol (csdfbmc/protocol.py) to a glue
// run as a command: one JSON request per line on its standard input, one
// JSON answer per line on its standard output. It asks only what needs no
// values, the lint request; the checker that walks states is csdfbmc.
package glue

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"github.com/Kuniwak/puml-parallel/v9/csdf/clause"
)

// Protocol is the protocol of atoms a glue is asked to speak.
const Protocol = 3

// Client is a glue started as a command, after its hello agreed on the
// protocol.
type Client struct {
	cmd      *exec.Cmd
	in       io.WriteCloser
	out      *bufio.Scanner
	language string
	supports []string
}

// StartCommandLine starts the glue the command line names, read into words
// by SplitCommand, its standard error written to stderr.
func StartCommandLine(line string, stderr io.Writer) (*Client, error) {
	words, err := SplitCommand(line)
	if err != nil {
		return nil, fmt.Errorf("glue.StartCommandLine: %w", err)
	}
	if len(words) == 0 {
		return nil, errors.New("glue.StartCommandLine: the command line of a glue is empty")
	}
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Stderr = stderr
	return Start(cmd)
}

// Start runs cmd as a glue and greets it; a glue that does not speak the
// protocol is refused and stopped.
func Start(cmd *exec.Cmd) (*Client, error) {
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("glue.Start: %w", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("glue.Start: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("glue.Start: cannot run the glue %s: %w", cmd.Path, err)
	}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
	c := &Client{cmd: cmd, in: in, out: sc}
	var hello struct {
		Language string   `json:"language"`
		Protocol int      `json:"protocol"`
		Supports []string `json:"supports"`
		Error    *string  `json:"error"`
	}
	if err := c.request(map[string]any{"op": "hello", "protocol": Protocol}, &hello); err != nil {
		c.Close()
		return nil, fmt.Errorf("glue.Start: %w", err)
	}
	switch {
	case hello.Error != nil:
		err = fmt.Errorf("the glue refused the hello: %s", *hello.Error)
	case hello.Protocol != Protocol:
		err = fmt.Errorf("the glue speaks protocol %d, not %d", hello.Protocol, Protocol)
	}
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("glue.Start: %w", err)
	}
	c.language, c.supports = hello.Language, hello.Supports
	return c, nil
}

// Language is the language the glue's hello named.
func (c *Client) Language() string { return c.language }

// Supports reports whether the glue's hello listed op among the optional
// requests it answers.
func (c *Client) Supports(op string) bool { return slices.Contains(c.supports, op) }

// Lint returns why the glue does not read atom in role, "guard" or "post",
// or "" when it does; an error is a glue that answered with one, or did not
// answer.
func (c *Client) Lint(atom clause.AtomWire, role string) (string, error) {
	var answer struct {
		OK    *bool   `json:"ok"`
		Why   string  `json:"why"`
		Error *string `json:"error"`
	}
	if err := c.request(map[string]any{"op": "lint", "atom": atom, "role": role}, &answer); err != nil {
		return "", fmt.Errorf("glue.Client.Lint: %w", err)
	}
	switch {
	case answer.Error != nil:
		return "", fmt.Errorf("glue.Client.Lint: an error: %s", *answer.Error)
	case answer.OK == nil:
		return "", errors.New("glue.Client.Lint: the answer to lint says neither ok nor an error")
	case *answer.OK:
		return "", nil
	case answer.Why == "":
		return "", errors.New("glue.Client.Lint: the answer to lint is not ok but says no why")
	}
	return answer.Why, nil
}

// Close ends the glue's input, which ends the glue, reads what it still
// writes, so that it is not left blocked on a full pipe, and waits for it;
// an error is a glue that did not end well.
func (c *Client) Close() error {
	c.in.Close()
	for c.out.Scan() {
	}
	if err := c.cmd.Wait(); err != nil {
		return fmt.Errorf("glue.Client.Close: %w", err)
	}
	return nil
}

// errEndedWithoutAnswering is a glue that ended before it answered, whether
// it is found while the request is written or while the answer is read.
var errEndedWithoutAnswering = errors.New("the glue ended without answering")

func (c *Client) request(request any, answer any) error {
	b, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			return errEndedWithoutAnswering
		}
		return fmt.Errorf("cannot write to the glue: %w", err)
	}
	if !c.out.Scan() {
		if err := c.out.Err(); err != nil {
			return fmt.Errorf("cannot read the glue's answer: %w", err)
		}
		return errEndedWithoutAnswering
	}
	if err := json.Unmarshal(c.out.Bytes(), answer); err != nil {
		return fmt.Errorf("the glue answered %q, not a JSON answer: %w", c.out.Text(), err)
	}
	return nil
}

// SplitCommand reads a command line into words as csdfbmc reads
// --evaluator (Python's shlex.split): words apart by blanks, single quotes
// keeping everything, double quotes keeping all but a backslash before " or
// \, and a backslash outside quotes keeping the next character.
func SplitCommand(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		case r == '\'':
			inWord = true
			j := slices.Index(rs[i+1:], '\'')
			if j < 0 {
				return nil, fmt.Errorf("glue.SplitCommand: %q has no closing single quote", line)
			}
			word.WriteString(string(rs[i+1 : i+1+j]))
			i += j + 1
		case r == '"':
			inWord = true
			closed := false
			for i++; i < len(rs); i++ {
				if rs[i] == '"' {
					closed = true
					break
				}
				if rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune(`"\`, rs[i+1]) {
					i++
				}
				word.WriteRune(rs[i])
			}
			if !closed {
				return nil, fmt.Errorf("glue.SplitCommand: %q has no closing double quote", line)
			}
		case r == '\\':
			inWord = true
			if i+1 == len(rs) {
				return nil, fmt.Errorf("glue.SplitCommand: %q ends in a backslash", line)
			}
			i++
			word.WriteRune(rs[i])
		default:
			inWord = true
			word.WriteRune(r)
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}
