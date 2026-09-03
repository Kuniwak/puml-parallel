package csdfhidecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Events []csdf.Event
	Bytes  []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfhide", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfhide [options] [file.puml|file.png]

Hides the given events of a Composable State Diagram following CSP hiding
semantics (P \ A): every edge carrying a hidden event becomes an internal tau
transition. The result is printed as PlantUML.
A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfhide -events 'insert;drop' path/to/file.puml
  $ csdfhide -events 'insert;drop' < path/to/file.puml
  $ csdfparallel -sync 'sync' a.puml b.puml | csdfhide -events 'sync' -
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		eventsFlag := flags.String("events", "", "semicolon-separated list of events to hide")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfhidecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfhidecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfhidecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		return &Options{
			Common: commonOpts,
			Events: tools.ParseEvents(*eventsFlag),
			Bytes:  bs,
		}, nil
	}
}
