package csdfparsecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// File is the input path. An empty string or "-" means standard input,
	// so `csdfparse f`, `csdfparse < f`, and `csdfparse - < f` are equivalent.
	File string
}

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfparse", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfparse [options] [file.puml|file.png]

Parses a Composable State Diagram and prints the parsed structure as JSON.
A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfparse path/to/file.puml
  $ csdfparse < path/to/file.puml
  $ csdfparse - < path/to/file.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfparsecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfparsecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		if flags.NArg() > 1 {
			return nil, fmt.Errorf("csdfparsecmd.NewParseOptionsFunc: too many arguments")
		}

		file := ""
		if flags.NArg() == 1 {
			file = flags.Arg(0)
		}

		return &Options{Common: commonOpts, File: file}, nil
	}
}
