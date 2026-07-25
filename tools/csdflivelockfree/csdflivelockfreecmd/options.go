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
	Target target.Name
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
predicates, which this tool leaves opaque. For isabelle and lean each distinct
predicate becomes a True placeholder definition named pred_<id> after its hash,
preceded by a comment carrying its original text; init and every transition then
get guard_L<line>/post_L<line> aliases of those placeholders. The theorem states
well-foundedness of the tau relation restricted to the states reachable from init
via the step relation, so that valuations the diagram can never enter cannot
falsify it. Filling the placeholders in and discharging the theorem is left to a
human or LLM. The IR sets structurally=true when no reachable "tau" cycle exists,
in which case no obligation is emitted at all. A file argument, a "-" argument,
and standard input are all equivalent.

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
		flags.StringVar(&tgt, "target", string(target.NameIRJSON), "output target: ir-json|isabelle|lean")

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

		if err := target.Validate(target.Name(tgt)); err != nil {
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdflivelockfreecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Target: target.Name(tgt), Bytes: bs}, nil
	}
}
