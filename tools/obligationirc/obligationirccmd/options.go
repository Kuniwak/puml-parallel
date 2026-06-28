package obligationirccmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

// Output targets selectable via -target.
const (
	TargetIRJSON   = "ir-json"
	TargetIsabelle = "isabelle"
	TargetLean     = "lean"
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

For isabelle and lean, each opaque Guard_L<line>/Post_L<line>/Init predicate
becomes a True placeholder definition preceded by a comment carrying its
original natural-language text, leaving the formalisation and proof to a human
or LLM. A file argument, a "-" argument, and standard input are all equivalent.

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

		var target string
		flags.StringVar(&target, "target", TargetIRJSON, "output target: ir-json|isabelle|lean")

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

		if err := validateTarget(target); err != nil {
			return nil, fmt.Errorf("obligationirccmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("obligationirccmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Target: target, Bytes: bs}, nil
	}
}

func validateTarget(target string) error {
	switch target {
	case TargetIRJSON, TargetIsabelle, TargetLean:
		return nil
	default:
		return fmt.Errorf("unknown -target %q (want ir-json, isabelle, or lean)", target)
	}
}
