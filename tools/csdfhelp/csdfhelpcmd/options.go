package csdfhelpcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/toolsdoc"
)

type Options struct {
	Common *tools.CommonOptions
	Short  bool
	// ToolNames is empty when every non-hidden tool should be shown.
	ToolNames []string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfhelp", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfhelp [options] [tool ...]

Prints the help of every tool in this repository as one Markdown document.
With no argument it shows every tool but csdfhelp itself; with tool names,
only those, csdfhelp included.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfhelp
  $ csdfhelp -short
  $ csdfhelp csdfparse csdfevents
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		short := flags.Bool("short", false, "print only a one-line summary per tool")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfhelpcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfhelpcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		registry := Registry()
		names := flags.Args()
		for _, name := range names {
			if _, ok := toolsdoc.Lookup(registry, name); !ok {
				return nil, fmt.Errorf("csdfhelpcmd.NewParseOptionsFunc: unknown tool: %s", name)
			}
		}

		return &Options{Common: commonOpts, Short: *short, ToolNames: names}, nil
	}
}
