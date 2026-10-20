package csdfhidecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common   *tools.CommonOptions
	Channels []string
	Bytes    []byte
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

Hides the given channels of a Composable State Diagram following CSP hiding
semantics (P \ {|c|}): every edge on a hidden channel becomes an internal tau
transition, whatever its fields; a hidden field is a value the tau chooses,
which the clauses still read by its name. The result is printed as PlantUML.
A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfhide -channels 'insert;drop' path/to/file.puml
  $ csdfhide -channels 'insert;drop' < path/to/file.puml
  $ csdfparallel -sync 'sync' a.puml b.puml | csdfhide -channels 'sync' -
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		channelsFlag := flags.String("channels", "", "semicolon-separated list of the channels to hide")

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

		channels, err := tools.ParseChannels(*channelsFlag)
		if err != nil {
			return nil, fmt.Errorf("csdfhidecmd.NewParseOptionsFunc: -channels: %w", err)
		}

		return &Options{
			Common:   commonOpts,
			Channels: channels,
			Bytes:    bs,
		}, nil
	}
}
