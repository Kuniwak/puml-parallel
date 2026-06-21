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

// NoFd is an invalid file descriptor. term.IsTerminal(int(NoFd)) reports false,
// so a stub stream created with it is treated as non-interactive.
const NoFd = ^uintptr(0)

// NewStdinFromFile widens a real file to a Stdin. *os.File already satisfies
// Stdin, so this is a static, type-checked conversion (no runtime assertion).
func NewStdinFromFile(f *os.File) Stdin { return f }

// NewStdoutFromFile widens a real file to a Stdout.
func NewStdoutFromFile(f *os.File) Stdout { return f }

type stubStdin struct {
	io.Reader
	fd uintptr
}

func (s stubStdin) Fd() uintptr { return s.fd }

// StubStdin builds a Stdin from any reader plus an explicit descriptor, letting
// callers decide whether the stream should look like a terminal.
func StubStdin(r io.Reader, fd uintptr) Stdin { return stubStdin{Reader: r, fd: fd} }

type stubStdout struct {
	io.Writer
	fd uintptr
}

func (s stubStdout) Fd() uintptr { return s.fd }

// StubStdout builds a Stdout from any writer plus an explicit descriptor.
func StubStdout(w io.Writer, fd uintptr) Stdout { return stubStdout{Writer: w, fd: fd} }

type ProcInout struct {
	Stdin  Stdin
	Stdout Stdout
	Stderr io.Writer
	Env    EnvFunc
}

func NewProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  NewStdinFromFile(os.Stdin),
		Stdout: NewStdoutFromFile(os.Stdout),
		Stderr: os.Stderr,
		Env:    os.Getenv,
	}
}

func StubProcInout() *ProcInout {
	return &ProcInout{
		Stdin:  StubStdin(io.NopCloser(strings.NewReader("")), NoFd),
		Stdout: StubStdout(io.Discard, NoFd),
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
		Stdin:  StubStdin(s.Stdin, NoFd),
		Stdout: StubStdout(s.Stdout, NoFd),
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
