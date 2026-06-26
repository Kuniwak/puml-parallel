package cli

import (
	"bytes"
	"io"
	"os"
	"sync"
)

type EnvFunc func(name string) string

func NewEnvFunc(env map[string]string) EnvFunc {
	return func(name string) string {
		return env[name]
	}
}

type FDReader interface {
	io.Reader
	Fd() uintptr
}

type FDWriter interface {
	io.Writer
	Fd() uintptr
}

type FDReaderStub struct {
	io.Reader
	fd uintptr
}

func (r FDReaderStub) Fd() uintptr { return r.fd }

func StubStdin(r io.Reader) FDReader { return FDReaderStub{r, 0} }

type FDWriterStub struct {
	io.Writer
	fd uintptr
}

func (s FDWriterStub) Fd() uintptr { return s.fd }

func StubStdout(w io.Writer) FDWriter { return FDWriterStub{w, 1} }
func StubStderr(w io.Writer) FDWriter { return FDWriterStub{w, 2} }

type ProcInout struct {
	Stdin  FDReader
	Stdout FDWriter
	Stderr FDWriter
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

func StubProcInout(r io.Reader, env map[string]string) *ProcInout {
	return &ProcInout{
		Stdin:  StubStdin(r),
		Stdout: StubStdout(io.Discard),
		Stderr: StubStdout(io.Discard),
		Env:    NewEnvFunc(env),
	}
}

type ProcInoutSpy struct {
	Stdin  io.Reader
	Stdout *LockedBuffer
	Stderr *LockedBuffer
	Env    map[string]string
}

func SpyProcInout() *ProcInoutSpy {
	return &ProcInoutSpy{
		Stdin:  &bytes.Buffer{},
		Stdout: NewLockedBuffer(),
		Stderr: NewLockedBuffer(),
		Env:    nil,
	}
}

func (s *ProcInoutSpy) New() *ProcInout {
	return &ProcInout{
		Stdin:  StubStdin(s.Stdin),
		Stdout: StubStdout(s.Stdout),
		Stderr: StubStderr(s.Stderr),
		Env:    NewEnvFunc(s.Env),
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

type LockedBuffer struct {
	mu     sync.Mutex
	buffer *bytes.Buffer
}

func NewLockedBuffer() *LockedBuffer {
	return &LockedBuffer{
		buffer: &bytes.Buffer{},
	}
}

func (b *LockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *LockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *LockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Len()
}
