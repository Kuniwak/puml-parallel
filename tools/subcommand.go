package tools

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/version"
)

type Subcommand struct {
	Name        string
	Description string
	CommandFunc cli.CommandFunc
}

// WriteCommandHelp renders the aligned command listing for a (sub)command group.
// It is shared by every group so the top-level command and its subcommand groups
// look identical.
func WriteCommandHelp(w io.Writer, name, desc string, cs []Subcommand) {
	fmt.Fprintf(w, "Usage: %s <command> [options]\n\n", name)
	if desc != "" {
		fmt.Fprintf(w, "%s\n\n", desc)
	}
	fmt.Fprintln(w, "Commands:")
	width := len("help")
	for _, c := range cs {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	for _, c := range cs {
		fmt.Fprintf(w, "  %-*s  %s\n", width, c.Name, c.Description)
	}
	fmt.Fprintf(w, "  %-*s  %s\n", width, "help", "show this help")
	fmt.Fprintf(w, "\nRun %q for a command's options and examples.\n", name+" help <command>")
}

// NewSubcommandFunc builds the dispatcher for a command group. It parses the
// common options, then routes the first positional to a subcommand. A missing
// command, -h/--help, or "help [command]" prints the group help and exits 0; an
// unknown command prints the group help to stderr and exits 1.
func NewSubcommandFunc(name, desc string, cs []Subcommand) cli.CommandFunc {
	return func(args []string, inout *cli.ProcInout) int {
		flags := flag.NewFlagSet(name, flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		var raw CommonRawOptions
		DeclareCommonOptions(flags, &raw)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				WriteCommandHelp(inout.Stdout, name, desc, cs)
				return 0
			}
			fmt.Fprintf(inout.Stderr, "Error: %s\n", UserFacingError(err, false))
			return 1
		}

		common, err := ValidateCommonOptions(&raw)
		if err != nil {
			fmt.Fprintf(inout.Stderr, "Error: %s\n", UserFacingError(err, false))
			return 1
		}
		if common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return 0
		}

		rest := flags.Args()
		if len(rest) == 0 {
			WriteCommandHelp(inout.Stdout, name, desc, cs)
			return 0
		}
		if rest[0] == "help" {
			return runHelp(name, desc, cs, rest[1:], inout)
		}
		for _, c := range cs {
			if c.Name == rest[0] {
				return c.CommandFunc(rest[1:], inout)
			}
		}
		fmt.Fprintf(inout.Stderr, "unknown command %q\n\n", rest[0])
		WriteCommandHelp(inout.Stderr, name, desc, cs)
		return 1
	}
}

// runHelp implements "help [command]": with no argument it prints the group
// help (exit 0); with a command it delegates to that command's own -h.
func runHelp(name, desc string, cs []Subcommand, args []string, inout *cli.ProcInout) int {
	if len(args) == 0 {
		WriteCommandHelp(inout.Stdout, name, desc, cs)
		return 0
	}
	for _, c := range cs {
		if c.Name == args[0] {
			return c.CommandFunc([]string{"-h"}, inout)
		}
	}
	fmt.Fprintf(inout.Stderr, "unknown command %q\n\n", args[0])
	WriteCommandHelp(inout.Stderr, name, desc, cs)
	return 1
}
