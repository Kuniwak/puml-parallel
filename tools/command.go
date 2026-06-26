package tools

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
)

// CommonOptionsCarrier is implemented by every tool's parsed options so generic
// command plumbing can read the common options (e.g., the log level for
// debug-gated error formatting).
type CommonOptionsCarrier interface {
	CommonOptions() *CommonOptions
}

// NewCommandFunc wraps a parser and main function into a cli.CommandFunc. It
// prints "Error: <msg>" to stderr and exits 1 when either the parser or the
// main function returns an error.
func NewCommandFunc[T any](parseOpts cli.ParseOptionsFunc[T], mainFunc cli.MainFunc[T]) cli.CommandFunc {
	return func(args []string, inout *cli.ProcInout) int {
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
