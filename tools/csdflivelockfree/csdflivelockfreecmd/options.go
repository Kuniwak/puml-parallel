package csdflivelockfreecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Target string
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

Compiles a livelock-freedom proof obligation for a Composable State Diagram to
the target selected by -target and exits 0:

  ir-json   a prover-agnostic JSON obligation IR (default)
  isabelle  an Isabelle/HOL proof-obligation skeleton
  lean      a Lean 4 proof-obligation skeleton

Whether the diagram is livelock free depends on the natural-language Guard/Post
predicates, which this tool leaves opaque as line-named symbols (Guard_L<line>,
Post_L<line>, Init); for isabelle and lean each becomes a True placeholder
definition preceded by a comment carrying its original text, leaving the
formalisation and proof to a human or LLM. The IR sets
structurally_livelock_free=true when no reachable "tau" cycle exists. A file
argument, a "-" argument, and standard input are all equivalent.

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

		var tgt string
		flags.StringVar(&tgt, "target", target.IRJSON, "output target: ir-json|isabelle|lean")

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

		if err := target.Validate(tgt); err != nil {
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Target: tgt, Bytes: bs}, nil
	}
}
