package csdfreplcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	File   string
	// Replay is the path of csdfbmc's JSON output to walk before the first
	// prompt, and Finding which of its findings; empty when none.
	Replay  string
	Finding int
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfrepl", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfrepl [options] <file.puml|file.png>

Interactively explores a Composable State Diagram.

With -replay, the session first walks the path csdfbmc --json recorded under
one of its findings (-finding picks which, numbered from 0 in the order the
output lists them: deadlocks, tau cycles, broken invariants, avoiding cycles,
empty posts, unconstrained posts), and the prompt opens at the state the
finding is at, with the path in the history.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfrepl examples/valid/vending_machine.puml
  $ csdfbmc diagram.json twins.py --json > findings.json
  $ csdfrepl -replay findings.json -finding 0 diagram.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		replay := flags.String("replay", "", "csdfbmc --json output whose finding to walk before the first prompt")
		finding := flags.Int("finding", 0, "with -replay, which finding to walk, numbered from 0")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfreplcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfreplcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		if flags.NArg() != 1 {
			return nil, fmt.Errorf("csdfreplcmd.NewParseOptionsFunc: exactly one CSDF file is required")
		}

		return &Options{Common: commonOpts, File: flags.Arg(0), Replay: *replay, Finding: *finding}, nil
	}
}
