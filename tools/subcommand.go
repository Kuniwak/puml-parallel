package tools

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/Kuniwak/puml-parallel/cli"
)

type Subcommand struct {
	Name        string
	Description string
	CommandFunc cli.CommandFunc
}

type SubcommandOptions struct {
	CommonOptions *CommonOptions
	Index         int
	Args          []string
}

func NewParseSubcommandOptions(name string, desc string, cs []Subcommand) cli.ParseOptionsFunc[*SubcommandOptions] {
	return func(args []string, inout *cli.ProcInout) (*SubcommandOptions, error) {
		flags := flag.NewFlagSet(name, flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			io.WriteString(w, `Usage: `)
			io.WriteString(w, name)
			io.WriteString(w, ` <command>

	`)
			io.WriteString(w, desc)
			io.WriteString(w, `
	Commands:
	`)
			for _, subCommand := range cs {
				io.WriteString(w, `  `)
				io.WriteString(w, subCommand.Name)
				io.WriteString(w, `
		`)
				io.WriteString(w, subCommand.Description)
				io.WriteString(w, "\n")
			}

			io.WriteString(w, `
	Options:
	`)
			flags.PrintDefaults()
		}

		var commonRawOptions CommonRawOptions
		DeclareCommonOptions(flags, &commonRawOptions)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &SubcommandOptions{CommonOptions: CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("tools.ParseSubcommandOptions: failed to parse arguments: %w", err)
		}

		commonOptions, err := ValidateCommonOptions(&commonRawOptions)
		if err != nil {
			return nil, fmt.Errorf("tools.ParseSubcommandOptions: failed to parse common options: %w", err)
		}

		if commonOptions.Version {
			return &SubcommandOptions{CommonOptions: CommonOptionsVersion}, nil
		}

		if flags.NArg() > 0 {
			subcmdName := flags.Arg(0)
			idx := slices.IndexFunc(cs, func(c Subcommand) bool {
				return c.Name == subcmdName
			})
			if idx >= 0 {
				return &SubcommandOptions{CommonOptions: commonOptions, Index: idx, Args: flags.Args()[1:]}, nil
			}
		}

		flags.Usage()
		return nil, errors.New("tools.parseSubcommandOptions: no such subcommand")
	}
}

func NewSubcommandFunc(name, desc string, cs []Subcommand) cli.CommandFunc {
	parseOptions := NewParseSubcommandOptions(name, desc, cs)
	return func(args []string, inout *cli.ProcInout) int {
		opts, err := parseOptions(args, inout)
		if err != nil {
			fmt.Fprintf(inout.Stderr, "tools.NewSubcommandFunc: %s\n", err)
			return 1
		}

		return cs[opts.Index].CommandFunc(opts.Args, inout)
	}
}
