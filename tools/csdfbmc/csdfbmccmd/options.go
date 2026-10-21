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

const usage = `Usage: csdfbmc [launcher options] <diagram.json> <checks.py> (--evaluator CMD | --evaluator-module MODULE) [checker options]

Runs csdfbmc, the bounded model checker, on a diagram as csdfparse prints it,
or an expansion as csdfpromote -format json prints it, with every clause read
through a glue of the evaluator protocol and the checks module of its
types, environment, ghosts and invariants. The checker is Python (standard library only,
3.11 or later); this binary carries its sources and runs them with the
python3 on the PATH, so a release needs no checkout. The launcher reads its
own options first and hands every argument after them to the checker as it
is.

Launcher options:
  -python PATH  the interpreter to run (default: $CSDFBMC_PYTHON, else python3 on the PATH)
  -v, -version  show version
  -h            show this help
  -debug        debug mode
  -silent       silent mode

Checker options (the checker's --help lists them in full):
  --evaluator CMD      read every clause through the glue CMD starts, which
                       speaks the evaluator protocol
  --evaluator-module MODULE
                       the same, with a glue written in Python loaded
                       in-process; "csdfbmc.python_glue DEFS.py..." reads
                       clauses written in Python. One of the two is needed.
  --max-states N       stop after this many states (default 100000)
  --seed N             choose the functions a clause not formalised denotes
                       from this seed, where nothing gives them
  --env NAME=true|false
                       give the function NAME the value true or false, over
                       the checks module's ENVIRONMENT (repeatable)
  --opaque error|true  what a clause not formalised yet means when the
                       environment does not decide it (default: refuse)
  --avoid EVENT        also report the cycles that never perform EVENT
  --within STATE       with --avoid, look among these control states only
  --first              stop at the first deadlock, broken invariant or unconstrained post
  --all                list every finding instead of one per shape with a count
  --por                take the moves of one independent instance at a time
                       (partial-order reduction)
  --symmetry           store one state per renaming of the instance IDs the
                       checks module declares SYMMETRIC
  --cone               leave out the variables that nothing deciding a step or
                       a property reads (cone of influence); same findings
  --json               print the result as JSON

The checks module defines TYPES (a type -> its finite values) and, when
wanted, TYPEOF (a name -> its type), ENVIRONMENT (the function a clause not
formalised denotes, named by its atom's text without its holes or by any
of its atoms -> True, False or a pure function of its holes' values in
order; @opaque is refused: write the clause as an atom), GHOSTS, INVARIANTS
and SYMMETRIC. A run reports its seed and the given functions, which
reproduce it.

"csdfbmc next <diagram.json> <checks.py> (--evaluator CMD | --evaluator-module
MODULE) (--initial | --state JSON --edge N)" answers what a REPL asks at a
value prompt: the values the clauses admit for the start state, or after the
N-th outgoing edge of the given runtime state, as
{"candidates": [{"params": {...}, "values": [...]}, ...]}. csdfrepl and
csdfrepld call it when given -checks.

The exit status is the checker's: 0 when nothing was found, 1 when a
deadlock, tau cycle, broken invariant or avoiding cycle was, 2 when the
inputs could not be read. The sources are unpacked once per version under
$CSDFBMC_CACHE, or the user cache directory.

Examples:
  $ csdfparse examples/bmc/withdraw_formal.puml > withdraw.json
  $ csdfbmc withdraw.json examples/bmc/withdraw_formal_defs.py \
      --evaluator-module "csdfbmc.python_glue examples/bmc/withdraw_formal_defs.py"
  $ csdfbmc wallet.json checks.py --evaluator-module "csdfbmc.python_glue checks.py" --avoid 確定 --within sending --json
  $ csdfpromote -format json GLOBAL.puml > global.json
  $ csdfbmc global.json checks.py --evaluator-module csdfbmc.python_glue --symmetry --por
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
