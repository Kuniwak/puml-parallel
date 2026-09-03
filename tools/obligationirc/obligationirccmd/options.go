package obligationirccmd

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
		flags := flag.NewFlagSet("obligationirc", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: obligationirc [options] [file.json|-]

Compiles the livelock-freedom proof-obligation IR (the JSON emitted by
csdflivelockfree) to the target selected by -target and exits 0:

  ir-json   the IR itself, re-encoded as JSON (default)
  isabelle  an Isabelle/HOL proof-obligation skeleton
  lean      a Lean 4 proof-obligation skeleton

For isabelle and lean, each distinct opaque predicate becomes an uninterpreted
declaration named pred_<id> after its hash - "opaque" in Lean, "consts" in
Isabelle - preceded by a TODO(csdf) marker and a comment carrying its original
natural-language text; init and every transition then get
guard_L<line>/post_L<line> aliases of those declarations. An omitted predicate is
the exception: its text really is "true", so it stays a definition. The theorem states
well-foundedness of the tau relation restricted to the states reachable from init
via the step relation, so that valuations the diagram can never enter cannot
falsify it. Replacing the declarations by real predicate bodies and discharging
the theorem is left to a human or LLM; treat an artifact that still carries a
TODO(csdf) marker as undischarged. A file argument, a "-" argument, and standard input are all
equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdflivelockfree path/to/file.puml | obligationirc -target lean
  $ csdflivelockfree path/to/file.puml | obligationirc -target isabelle
  $ obligationirc -target ir-json path/to/ir.json
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
			return nil, fmt.Errorf("obligationirccmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("obligationirccmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		if err := target.Validate(target.Name(tgt)); err != nil {
			return nil, fmt.Errorf("obligationirccmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("obligationirccmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Target: target.Name(tgt), Bytes: bs}, nil
	}
}
