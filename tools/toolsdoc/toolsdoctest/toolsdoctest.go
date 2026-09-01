// Package toolsdoctest provides stub command functions for testing a tool
// catalog, so every package that builds one can share the same doubles.
package toolsdoctest

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
)

// StubHelp returns a command function that writes usage to stderr and succeeds,
// as every tool built with tools.NewCommandFunc does for -h.
func StubHelp(usage string) cli.CommandFunc {
	return func(args []string, inout *cli.ProcInout) int {
		fmt.Fprint(inout.Stderr, usage)
		return 0
	}
}

// StubFailingHelp returns a command function that writes nothing and fails.
func StubFailingHelp() cli.CommandFunc {
	return func(args []string, inout *cli.ProcInout) int {
		return 1
	}
}
