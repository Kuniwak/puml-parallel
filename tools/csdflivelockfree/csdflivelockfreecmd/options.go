package csdflivelockfreecmd

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
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
			io.WriteString(w, `Usage: csdflivelockfree [options] [file.puml|file.png]

Compiles a livelock-freedom proof obligation for a Composable State Diagram to
the target selected by -target and exits 0:

  ir-json   a prover-agnostic JSON obligation IR (default)
  isabelle  an Isabelle/HOL proof-obligation skeleton
  lean      a Lean 4 proof-obligation skeleton

The clauses are read as formulas (docs/SYNTAX.md) and spelled in the target,
not left opaque: a builtin is an equality or a csdf_* predicate, an ∃ a
quantifier relativized to its type, an entry of a map csdf_index and a field
field_<name>. What stays uninterpreted is the environment: each atom becomes a
predicate atom_<text> applied to its holes, each function fn_<name> and each type
type_<name>, declared - "opaque" in Lean, "consts" in Isabelle - under a
TODO(csdf) marker and a comment carrying the text it was named by. A state
variable and a received field are of the type the diagram declares of them,
and one of the type any is quantified without restriction. Nothing is ever defined as True on the
diagram's behalf, because that is not a placeholder but a different diagram.
The builtins are defined, a map being a ValMap of the value type, and what the
csdf_* predicates mean is proved from the definitions as csdf_lemma_<i>, so the
theorem rests on no axiom. The theorem states well-foundedness of the tau relation restricted to
the states reachable from init via the step relation, so that valuations the
diagram can never enter cannot falsify it. Formalising the environment and
discharging the theorem is left to a human or LLM; treat an artifact that still
carries a TODO(csdf) marker as undischarged. CSDF names are not prover
identifiers, so every name is encoded behind a prefix of its kind. The IR sets
structurally=true when no reachable "tau" cycle exists, in which case no
obligation is emitted at all.

An assume note of the diagram ("note as <id>" holding "assume <text>", see
docs/SYNTAX.md) is what the diagram assumes of its runs as a whole, such as a
fairness assumption no guard can state. The IR lists them under "assumptions";
for isabelle and lean each becomes an uninterpreted proposition named
assume_<id>, declared like an atom under a TODO(csdf) marker, and the
theorem is stated under those hypotheses. A file argument, a "-" argument,
and standard input are all equivalent.

The isabelle theory imports the libraries the builtins rest on, so the ROOT of
the session building it, saved as Livelock_Obligation.thy, declares their
sessions:

`+isabelle.Root("Csdf_Obligation", "HOL", "Livelock_Obligation")+`
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
