package csdfrefinementcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Mode   obligationir.IRRefinementMode
	Target target.Name
	Spec   []byte
	Impl   []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

// ParseMode maps a -m spelling to the model the obligation is stated in. Both the
// short and the long name are accepted; there is no default, because which model
// a refinement holds in is a different claim in each case.
func ParseMode(s string) (obligationir.IRRefinementMode, error) {
	switch s {
	case "t", "trace":
		return obligationir.IRRefinementModeTrace, nil
	case "f", "stable-failure":
		return obligationir.IRRefinementModeStableFailure, nil
	case "fd", "failures-divergence":
		return obligationir.IRRefinementModeFailuresDivergence, nil
	case "":
		return "", errors.New("-m is required (want t|trace, f|stable-failure, or fd|failures-divergence)")
	default:
		return "", fmt.Errorf("unknown -m %q (want t|trace, f|stable-failure, or fd|failures-divergence)", s)
	}
}

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfrefinement", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfrefinement -m <model> [options] abs.puml detail.puml

Compiles a refinement proof obligation for two Composable State Diagrams to the
target selected by -target and exits 0. The first diagram is the Spec (the
abstract side) and the second is the Impl (the concrete side); the obligation
states that every behaviour of the Impl is allowed by the Spec, which is the
direction FDR checks.

Models selected by -m, which is mandatory:

  t  | trace                the traces of the Impl are traces of the Spec
  f  | stable-failure       likewise for stable failures, which subsumes traces
  fd | failures-divergence  likewise for failures and divergences

Targets selected by -target:

  ir-json   a prover-agnostic JSON obligation IR (default)
  isabelle  an Isabelle/HOL skeleton importing CSP-Prover
  lean      a Lean 4 skeleton importing lean-csp-prover

Whether the refinement holds depends on the natural-language Guard/Post
predicates, which this tool leaves opaque, so the exit status never encodes the
verdict. Each distinct predicate becomes an uninterpreted declaration named
pred_<id> after its hash - "opaque" in Lean, "consts" in Isabelle - carrying its
original text and a TODO(csdf) marker as comments; treat an artifact that still
carries a marker as undischarged. Nothing is ever defined as True on the
diagram's behalf, because that is not a placeholder but a different diagram, in
which every guard fires. An omitted predicate is the exception: its text really
is "true", so it stays a definition.

The name is shared by both diagrams, and with csdflivelockfree's output for the
same diagram, so formalising a predicate once serves every obligation it appears
in - unless two predicates collide under the hash, in which case the later one
takes the next free id and the two tools may name it differently. Everything
named after a source location is side-qualified (guard_S_L<line>, post_I_L<line>,
...), because line numbers collide between the two files. CSDF names are not
prover identifiers, so events, state ids and variable names are encoded, and the
theory opens with a table giving the originals of the names that had to be
encoded.

CSP-Prover has no failures-divergences model, so -m fd emits the standard
reduction instead: a divergence-freedom obligation per side - the csdflivelockfree
obligation inlined, plus an initialisability obligation, since a diagram that
cannot start denotes DIV and DIV diverges on the empty trace - plus the
stable-failures refinement. Note that the Spec-side obligations are stronger than
FD refinement needs: DIV is the bottom of the FD model, so Spec ⊑FD Impl holds
even for a diverging Spec. Reducing to <=F cannot see that, so -m fd cannot
decide a refinement whose Spec side diverges.

Checking the emitted theory needs the corresponding library: Isabelle2020 with
CSP-Prover for isabelle, and lean-csp-prover for lean. The two skeletons mirror
each other declaration for declaration, so they can be read side by side.

At most one argument may be "-", which reads that diagram from standard input.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfrefinement -m t abs.puml detail.puml
  $ csdfrefinement -m fd -target isabelle abs.puml detail.puml
  $ csdfparallel a.puml b.puml | csdfrefinement -m f abs.puml -
`)
		}

		var mode string
		flags.StringVar(&mode, "m", "", "model: t|trace, f|stable-failure, or fd|failures-divergence (required)")

		var tgt string
		flags.StringVar(&tgt, "target", string(target.NameIRJSON), "output target: ir-json|isabelle|lean")

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfrefinementcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfrefinementcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		m, err := ParseMode(mode)
		if err != nil {
			return nil, fmt.Errorf("csdfrefinementcmd.NewParseOptionsFunc: %w", err)
		}

		if err := target.ValidateRefinement(target.Name(tgt)); err != nil {
			return nil, fmt.Errorf("csdfrefinementcmd.NewParseOptionsFunc: %w", err)
		}

		bss, err := tools.ValidateArgsAsTwoFilePaths(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfrefinementcmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		return &Options{
			Common: commonOpts,
			Mode:   m,
			Target: target.Name(tgt),
			Spec:   bss[0],
			Impl:   bss[1],
		}, nil
	}
}
