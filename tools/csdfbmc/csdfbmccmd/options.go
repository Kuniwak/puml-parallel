package csdfbmccmd

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

// Options are the launcher's own; everything after them is the checker's.
type Options struct {
	Common *tools.CommonOptions
	// Python is the interpreter to run, when the user named one.
	Python string
	// Args go to the checker as they are.
	Args []string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

const usage = `Usage: csdfbmc [launcher options] <diagram.json> <twins.py> [checker options]

Runs csdfbmc, the bounded model checker, on a diagram as csdfparse prints it,
or an expansion as csdfpromote -format json prints it, and the Python module
holding the formal twins of its clauses. The checker is
Python (standard library only, 3.11 or later); this binary carries its sources
and runs them with the python3 on the PATH, so a release needs no checkout.
The launcher reads its own options first and hands every argument after them
to the checker as it is.

Launcher options:
  -python PATH  the interpreter to run (default: $CSDFBMC_PYTHON, else python3 on the PATH)
  -v, -version  show version
  -h            show this help
  -debug        debug mode
  -silent       silent mode

Checker options (the checker's --help lists them in full):
  --local PATH=TWINS   the twins of the local diagram at PATH, for an expansion
                       of a promoted diagram (repeatable)
  --evaluator CMD      read every clause through the glue CMD starts, which
                       speaks the evaluator protocol (no twins keyed by prose)
  --evaluator-module MODULE
                       the same, with a glue written in Python loaded
                       in-process; "csdfbmc.python_glue DEFS.py..." reads
                       clauses written in Python
  --max-states N       stop after this many states (default 100000)
  --opaque error|true  what a clause without a twin means (default: refuse)
  --avoid EVENT        also report the cycles that never perform EVENT
  --within STATE       with --avoid, look among these control states only
  --first              stop at the first deadlock, broken invariant or unconstrained post
  --all                list every finding instead of one per shape with a count
  --por                take the moves of one independent instance at a time
                       (partial-order reduction)
  --symmetry           store one state per renaming of the instance IDs the
                       twins module declares SYMMETRIC
  --cone               leave out the variables that nothing deciding a step or
                       a property reads (cone of influence); same findings
  --json               print the result as JSON

"csdfbmc next <diagram.json> <twins.py> (--initial | --state JSON --edge N)"
answers what a REPL asks at a value prompt: the values the twins admit for
the start state, or after the N-th outgoing edge of the given runtime state,
as {"candidates": [{"params": {...}, "values": [...]}, ...]}. csdfrepl and
csdfrepld call it when given -twins.

The exit status is the checker's: 0 when nothing was found, 1 when a
deadlock, tau cycle, broken invariant or avoiding cycle was, 2 when the
inputs could not be read. The sources are unpacked once per version under
$CSDFBMC_CACHE, or the user cache directory.

Examples:
  $ csdfparse examples/bmc/withdraw.puml > withdraw.json
  $ csdfbmc withdraw.json examples/bmc/withdraw_twins.py
  $ csdfbmc wallet.json wallet_twins.py --avoid 確定 --within sending --json
  $ csdfbmc withdraw_formal.json defs.py --evaluator-module "csdfbmc.python_glue defs.py"
  $ csdfpromote -format json GLOBAL.puml > global.json
  $ csdfbmc global.json global_twins.py --local states/LOCAL.puml=local_twins.py
`

// NewParseOptionsFunc reads the launcher's options by hand rather than with
// flag: the checker's options start with "--" too, and flag would have to be
// taught every one of them to leave them alone.
func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		var raw tools.CommonRawOptions
		opts := &Options{}
		i := 0
		for ; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "-h" || a == "-help" || a == "--help":
				fmt.Fprint(inout.Stderr, usage)
				return &Options{Common: tools.CommonOptionsHelp}, nil
			case a == "-v":
				raw.ShortVersion = true
			case a == "-version":
				raw.Version = true
			case a == "-debug":
				raw.Debug = true
			case a == "-silent":
				raw.Silent = true
			case a == "-python":
				if i+1 >= len(args) {
					return nil, fmt.Errorf("csdfbmccmd.NewParseOptionsFunc: -python needs a path")
				}
				i++
				opts.Python = args[i]
			case strings.HasPrefix(a, "-python="):
				opts.Python = strings.TrimPrefix(a, "-python=")
			default:
				goto rest // the rest is the checker's
			}
		}
	rest:
		common, err := tools.ValidateCommonOptions(&raw)
		if err != nil {
			return nil, fmt.Errorf("csdfbmccmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		opts.Common = common
		if common.Version {
			return opts, nil
		}
		opts.Args = args[i:]
		if len(opts.Args) == 0 {
			fmt.Fprint(inout.Stderr, usage)
			return nil, fmt.Errorf("csdfbmccmd.NewParseOptionsFunc: too few arguments")
		}
		return opts, nil
	}
}
