package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type EnvFunc func(name string) string

func NewEnvFunc(env map[string]string) EnvFunc {
	return func(name string) string {
		return env[name]
	}
}

type Stdin interface {
	io.Reader
	Fd() uintptr
}

type Stdout interface {
	io.Writer
	Fd() uintptr
}

// noFd is an invalid descriptor; term.IsTerminal(int(noFd)) == false, so streams
// that are not backed by a file are treated as non-interactive.
const noFd = ^uintptr(0)

type stdinWithoutFd struct{ io.Reader }

func (stdinWithoutFd) Fd() uintptr { return noFd }

type stdoutWithoutFd struct{ io.Writer }

func (stdoutWithoutFd) Fd() uintptr { return noFd }

// NewStdin adapts an arbitrary reader (tests, in-memory streams) into a Stdin.
// A reader that already exposes Fd() (e.g. *os.File) is passed through unchanged.
func NewStdin(r io.Reader) Stdin {
	if s, ok := r.(Stdin); ok {
		return s
	}
	return stdinWithoutFd{r}
}

// NewStdout adapts an arbitrary writer into a Stdout. A writer that already
// exposes Fd() (e.g. *os.File) is passed through unchanged.
func NewStdout(w io.Writer) Stdout {
	if s, ok := w.(Stdout); ok {
		return s
	}
	return stdoutWithoutFd{w}
}

type ProcInout struct {
	Stdin  Stdin
	Stdout Stdout
	Stderr io.Writer
	Env    EnvFunc
}

func NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    os.Getenv,
	}
}

func StubProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  NewStdin(io.NopCloser(strings.NewReader(""))),
		Stdout: NewStdout(io.Discard),
		Stderr: io.Discard,
		Env:    func(name string) string { return "" },
	}
}

type ProcInoutSpy struct {
	Stdin  io.Reader
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
	Env    map[string]string
}

func (s *ProcInoutSpy) NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  NewStdin(s.Stdin),
		Stdout: NewStdout(s.Stdout),
		Stderr: s.Stderr,
		Env:    NewEnvFunc(s.Env),
	}
}

func SpyProcInout(stdin ...string) *ProcInoutSpy {
	return &ProcInoutSpy{
		Stdin:  strings.NewReader(strings.Join(stdin, "\n")),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Env:    make(map[string]string),
	}
}

type CommandFunc func(args []string, inout *ProcInout) int
type MainFunc[T any] func(opts T, procInout *ProcInout) error
type ParseOptionsFunc[T any] func(args []string, inouts *ProcInout) (T, error)

func (c CommandFunc) Run() {
	args := os.Args[1:]
	exitStatus := c(args, NewProcInout())
	os.Exit(exitStatus)
}

func NewCommandFunc[T any](parseOpts ParseOptionsFunc[T], mainFunc MainFunc[T]) CommandFunc {
	return func(args []string, inout *ProcInout) int {
		opts, err := parseOpts(args, inout)
		if err != nil {
			fmt.Fprintf(inout.Stderr, "Error: %s\n", err)
			return 1
		}

		if err := mainFunc(opts, inout); err != nil {
			fmt.Fprintf(inout.Stderr, "Error: %s\n", err)
			return 1
		}

		return 0
	}
}
