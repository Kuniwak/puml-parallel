package csdfparsecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Bytes  []byte
	// Path is the path of the file read, "" for standard input.
	Path string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfparse", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfparse [options] [file.puml|file.png]

Parses a Composable State Diagram, plain or global, and prints its concrete
syntax tree as JSON ("kind": "csdf-syntax-tree", version 1; docs/SYNTAX.md):
every element in the order written, comments, pragmas, CSDF-IGNORE regions,
notes and <<promote>> blocks included, each with its line and column, and
each clause as its own syntax tree. Nothing is expanded or resolved: a
directive is printed as written, and an !include as its path.
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

		path, bs, err := tools.ValidateArgsAsFileInput(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfparsecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Bytes: bs, Path: path}, nil
	}
}
