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

// abbreviations are the short spellings of the models -m takes.
var abbreviations = map[string]obligationir.IRRefinementMode{
	"t":  obligationir.IRRefinementModeTrace,
	"f":  obligationir.IRRefinementModeStableFailure,
	"fd": obligationir.IRRefinementModeFailuresDivergence,
}

// ParseMode maps a -m spelling to the model the obligation is stated in. Both the
// short and the long name are accepted; there is no default, because which model
// a refinement holds in is a different claim in each case.
func ParseMode(s string) (obligationir.IRRefinementMode, error) {
	if s == "" {
		return "", errors.New("-m is required (want t|trace, f|stable-failure, or fd|failures-divergence)")
	}
	if m, ok := abbreviations[s]; ok {
		return m, nil
	}
	m, err := obligationir.ParseIRRefinementMode(s)
	if err != nil {
		return "", fmt.Errorf("unknown -m %q (want t|trace, f|stable-failure, or fd|failures-divergence)", s)
	}
	return m, nil
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

The diagrams are read as formulas (docs/SYNTAX.md) and their clauses spelled in
the target as csdflivelockfree spells them: only the environment - each atom
(atom_<text>), function (fn_<name>) and type (type_<name>) - is declared
uninterpreted, under a TODO(csdf) marker, once for both diagrams. Whether the
refinement holds depends on it, so the exit status never encodes the verdict;
treat an artifact that still carries a marker as undischarged. Nothing is ever
defined as True on the diagram's behalf, because that is not a placeholder but a
different diagram, in which every guard holds.

An event carries the value of each of its fields, in order: an event of a
channel is the channel applied to them, so inc?k with k = 1 and with k = 2 are
two events. Each edge of a state is an external choice of the events it can
perform - its received fields of their types, for which some values after it
pass its guard and post - followed by an internal choice of those values after
it; an end edge is SKIP when its guard holds. A channel with one number of
fields in one place and another elsewhere is refused. The refinement is stated
under the axioms of the csdf_* predicates, as csdf_axiom_<i>, alone. The assume
notes of a diagram (see docs/SYNTAX.md), as assume_<id>_S or assume_<id>_I, are
hypotheses only of that side's own lemmas: initialisable_S/_I, stated in every
model since a side that cannot start is DIV and makes the refinement vacuous, and
under -m fd its livelock freedom. An assumption of the Impl could make the
refinement hold vacuously, and one of the Spec, of liveness, is no claim traces
or stable failures can state.

CSP-Prover has no failures-divergences model, so -m fd emits the standard
reduction instead: a divergence-freedom obligation per side - the csdflivelockfree
obligation inlined, beside the initialisability obligation, since DIV diverges
on the empty trace - plus the
stable-failures refinement. Note that the Spec-side obligations are stronger than
FD refinement needs: DIV is the bottom of the FD model, so Spec ⊑FD Impl holds
even for a diverging Spec. Reducing to <=F cannot see that, so -m fd cannot
decide a refinement whose Spec side diverges.

Checking the emitted theory needs the corresponding library: CSP-Prover for
isabelle (the Isabelle2025 fork at github.com/Kuniwak/CSP-Prover; build its
CSP_F session, then extend it from the obligation's ROOT), and lean-csp-prover
for lean (github.com/Kuniwak/lean-csp-prover). The two skeletons mirror
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
