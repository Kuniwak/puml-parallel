package csdfreplcmd

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
	File   string
	// Replay is the path of csdfbmc's JSON output to walk before the first
	// prompt, and Finding which of its findings; empty when none.
	Replay  string
	Finding int
	// Checker is what csdfbmc reads to let the value prompt suggest values,
	// and DiagramJSON the JSON it reads when the file is an expansion.
	Checker     bmc.Checker
	DiagramJSON string
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

With -checks and a glue (-evaluator or -evaluator-module), every value prompt
also shows the values the clauses admit for the pending transition, as
csdfbmc reads them through the glue, numbered, and "#N" enters the N-th;
csdfbmc must be on the PATH or beside this binary. -diagram-json names the
JSON (csdfpromote -format json, for an expansion) the checker reads instead of the
file's own parse.

-seed and -env are csdfbmc's --seed and --env: the prompt suggests what
csdfbmc walks with the same seed and the same given functions. Neither has a
default; a function the checks module declares that nothing decides is
refused, as csdfbmc refuses it. With -replay, the report's seed and the
values its --env gave are taken where -seed and -env say nothing; a -seed
or an -env value other than the report's, an -env of a function the
report's --env did not give, or a report that records no environment or
no env (one csdfbmc v1.8.0 and earlier wrote), is refused; make it again.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfrepl examples/valid/vending_machine.puml
  $ csdfbmc diagram.json checks.py --evaluator-module csdfbmc.python_glue --json > findings.json
  $ csdfrepl -replay findings.json -finding 0 diagram.puml
  $ csdfrepl -checks examples/bmc/withdraw_formal_defs.py \
      -evaluator-module "csdfbmc.python_glue examples/bmc/withdraw_formal_defs.py" examples/bmc/withdraw_formal.puml
  $ csdfrepl -checks checks.py -evaluator-module csdfbmc.python_glue -seed 7 -env '$_0 が 正=true' diagram.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		replay := flags.String("replay", "", "csdfbmc --json output whose finding to walk before the first prompt")
		finding := flags.Int("finding", 0, "with -replay, which finding to walk, numbered from 0")
		var checker bmc.Checker
		tools.DeclareCheckerOptions(flags, &checker)
		diagramJSON := flags.String("diagram-json", "", "with -checks, the JSON csdfbmc reads (an expansion); default: the file's own parse")

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

		if err := tools.ValidateCheckerOptions(checker); err != nil {
			return nil, fmt.Errorf("csdfreplcmd.NewParseOptionsFunc: %w", err)
		}
		if flags.NArg() != 1 {
			return nil, fmt.Errorf("csdfreplcmd.NewParseOptionsFunc: exactly one CSDF file is required")
		}

		return &Options{Common: commonOpts, File: flags.Arg(0), Replay: *replay, Finding: *finding,
			Checker: checker, DiagramJSON: *diagramJSON}, nil
	}
}
