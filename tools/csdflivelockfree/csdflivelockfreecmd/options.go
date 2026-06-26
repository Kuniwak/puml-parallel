package csdflivelockfreecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Bytes  []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdflivelockfree", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdflivelockfree [options] [file.puml|file.png]

Verifies that a Composable State Diagram is livelock free, i.e. has no cycle
reachable from the start state consisting entirely of internal "tau" transitions.
Prints "livelock free" and exits 0 when free; otherwise prints a witness and exits 1.
A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdflivelockfree path/to/file.puml
  $ csdflivelockfree < path/to/file.puml
  $ csdfparallel a.puml b.puml | csdflivelockfree -
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Bytes: bs}, nil
	}
}
