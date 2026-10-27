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
	// Checker is what csdfbmc reads to let the value prompt suggest values.
	Checker bmc.Checker
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

Interactively explores a Composable State Diagram whose guards and posts
are clauses (docs/SYNTAX.md, "Clauses"); a clause of the old syntax is
refused. Each value prompt shows the guard and the post as the clause
printer writes them; enter the values of the state's variables as a JSON
array in declaration order. A map of a family's instances is a JSON object
from IDs to {"at": <state>, "fields": {...}}, at a state of the family other
than its start state and with exactly that state's variables as fields.

A global diagram holding promotion directives (docs/PROMOTION.md) is
explored as its expansion, read as csdfpromote reads it, families included,
each !include read relative to the file; the promotion's warnings are
printed on the standard error. A diagram without them reads unchanged.

Without -checks, values a builtin of the guard or the post (@eq, @ne, @in,
@notin, @empty, @update, @frame, @at) is false of, with the previous state's
values, are refused; a builtin reading an event's field or an environment
function is left alone, and atoms are not evaluated, for only a glue reads
them. With -checks, values csdfbmc next does not admit are refused.

With -replay, the session first walks the path csdfbmc --json recorded under
one of its findings (-finding picks which, numbered from 0 in the order the
output lists them: deadlocks, tau cycles, broken invariants, avoiding cycles,
empty posts, unconstrained posts), and the prompt opens at the state the
finding is at, with the path in the history.

With -checks and a glue (-evaluator or -evaluator-module), every value prompt
also shows the values the clauses admit for the pending transition, as
csdfbmc reads them through the glue, numbered, and "#N" enters the N-th;
csdfbmc must be on the PATH or beside this binary.

-env is csdfbmc's --env: the prompt suggests what csdfbmc walks with the
same given functions. It has no default; a function the checks module
declares that nothing gives is refused, as csdfbmc refuses it. With
-replay, the values the report's --env gave are taken where -env says
nothing; an -env value other than the report's, an -env of a function the
report's --env did not give, or a report that records no environment or
no env (one that a csdfbmc before 3.0.0 wrote), is refused; make it again.

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
  $ csdfrepl -checks checks.py -evaluator-module csdfbmc.python_glue -env '$_0 が 正=true' diagram.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		replay := flags.String("replay", "", "csdfbmc --json output whose finding to walk before the first prompt")
		finding := flags.Int("finding", 0, "with -replay, which finding to walk, numbered from 0")
		var checker bmc.Checker
		tools.DeclareCheckerOptions(flags, &checker)

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
			Checker: checker}, nil
	}
}
