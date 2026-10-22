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

For isabelle and lean the output is what csdflivelockfree -target writes for the
same diagram: the clauses spelled as formulas, the environment (atoms, functions,
types) declared under TODO(csdf) markers, and the theorem stated under the axioms
of the csdf_* predicates and the diagram's assumptions. Formalising the
environment and discharging the theorem is left to a human or LLM; treat an
artifact that still carries a TODO(csdf) marker as undischarged. A file argument, a "-" argument, and standard input are all
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
