package tools

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
)

type exitOptions struct{ common *CommonOptions }

func (o *exitOptions) CommonOptions() *CommonOptions { return o.common }

// A tool that hands its work to another process reports that process's exit
// status as its own, without an "Error:" line of its own when the process has
// already said what went wrong.
func TestNewCommandFuncReturnsTheExitStatusAnErrorCarries(t *testing.T) {
	cases := map[string]struct {
		err        error
		wantStatus int
		wantStderr string
	}{
		"a plain error exits 1 and is printed":    {err: errors.New("boom"), wantStatus: 1, wantStderr: "Error: boom\n"},
		"an exit status is returned as is":        {err: &ExitStatusError{Status: 2}, wantStatus: 2, wantStderr: ""},
		"a wrapped exit status is found":          {err: fmt.Errorf("csdfbmccmd: %w", &ExitStatusError{Status: 3}), wantStatus: 3, wantStderr: ""},
		"an exit status with a message prints it": {err: &ExitStatusError{Status: 2, Message: "python3 is not on the PATH"}, wantStatus: 2, wantStderr: "Error: python3 is not on the PATH\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			parse := func(args []string, inout *cli.ProcInout) (*exitOptions, error) { return &exitOptions{}, nil }
			run := func(opts *exitOptions, inout *cli.ProcInout) error { return c.err }
			spy := cli.SpyProcInout()
			status := NewCommandFunc(parse, run)(nil, spy.New())
			if status != c.wantStatus {
				t.Errorf("exit status = %d, want %d", status, c.wantStatus)
			}
			if got := spy.Stderr.String(); got != c.wantStderr {
				t.Errorf("stderr = %q, want %q", got, c.wantStderr)
			}
		})
	}
}
