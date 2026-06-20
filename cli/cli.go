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

type ProcInout struct {
	Stdin  io.Reader
	Stdout io.Writer
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
		Stdin:  io.NopCloser(strings.NewReader("")),
		Stdout: io.Discard,
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
		Stdin:  s.Stdin,
		Stdout: s.Stdout,
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
