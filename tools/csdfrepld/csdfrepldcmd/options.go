package csdfrepldcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Sock   string
	// Checker is what csdfbmc reads to let every session's value prompts
	// suggest values.
	Checker bmc.Checker
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfrepld", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfrepld [options]

Runs the CSDF REPL daemon. It listens on a Unix domain socket and holds
interactive exploration sessions in memory, which the csdfreplcmd client drives.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
The socket path defaults to $%s, then $XDG_RUNTIME_DIR/%s, then <tmp>/%s.

Examples:
  $ csdfrepld
  $ csdfrepld -sock /tmp/csdfrepld.sock
  $ csdfrepld -checks checks.py -evaluator-module csdfbmc.python_glue
  $ csdfrepld -checks checks.py -evaluator-module csdfbmc.python_glue -seed 7

With -checks and a glue, value prompts suggest the values the clauses admit,
as csdfbmc reads them, and statevar -pick enters one. -seed and -env are
csdfbmc's --seed and --env, for every session; neither has a default. A
replay of a report made with another seed, or with a seed not given here,
is refused.
`, tools.SocketEnv, tools.SocketName, tools.SocketName)
		}

		var sock string
		flags.StringVar(&sock, "sock", "", "path of the Unix domain socket to listen on")
		var checker bmc.Checker
		tools.DeclareCheckerOptions(flags, &checker)

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfrepldcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfrepldcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		if err := tools.ValidateCheckerOptions(checker); err != nil {
			return nil, fmt.Errorf("csdfrepldcmd.NewParseOptionsFunc: %w", err)
		}

		return &Options{Common: commonOpts, Sock: sock, Checker: checker}, nil
	}
}
