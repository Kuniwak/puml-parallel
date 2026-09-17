package csdftracechkcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common  *tools.CommonOptions
	Match   MatchName
	Diagram []byte
	// Traces are the trace TSVs as read; they are parsed by the main function.
	Traces []tools.FileInput
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdftracechk", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdftracechk [options] <diagram.puml|diagram.png|-> <trace.tsv> [trace.tsv ...]

Checks whether each trace is a trace of the Composable State Diagram, and
writes a Markdown report to standard output. A trace is a TSV whose header is
the single column "event" and whose every other row is one visible event; the
internal event "tau" cannot appear in it.

The check first takes every guard as true. If the diagram cannot perform some
event even then, the report says which event, from which states, and which
events were enabled there, and the exit status is 1. Otherwise the report lists,
for every path the diagram can take along the trace (tau steps included), the
guards and postconditions along it, and states the obligation

  ∃ x0 x1 ... xn. post_0(x0) ∧ guard_1(x0) ∧ post_1(x0, x1) ∧ guard_2(x1) ∧ ...

that has to be satisfiable for at least one path for the trace to be a trace in
fact, followed by a prompt asking a reader (a person or an LLM) to decide it.
The predicates are natural language, so this tool never decides that itself;
the exit status is 0 when every trace passed the all-guards-true check.

Events are free-form text and their notation is not fixed, so the way a trace
event is looked up in the diagram is one of two simple rules chosen by -match:

  exact   the identical text (default)
  prefix  the trace event is a prefix of the diagram event, so a trace whose
          parameters were stripped ("insert") matches the label "insert(coin)"

Anything richer is best done to the trace beforehand, e.g. with sed or qhs. A
trace event matching several edges is nondeterminism, and yields one path per
edge. The diagram may be "-" for standard input; traces must be files.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdftracechk examples/valid/vending_machine.puml trace.tsv
  $ csdftracechk -match prefix examples/valid/vending_machine.puml trace1.tsv trace2.tsv
  $ csdfparallel -sync 'insert(coin)' a.puml b.puml | csdftracechk - trace.tsv
`)
		}

		var match string
		flags.StringVar(&match, "match", string(MatchNameExact), "how a trace event is looked up in the diagram: exact|prefix")

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		m, err := ParseMatchName(match)
		if err != nil {
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: %w", err)
		}

		rest := flags.Args()
		if len(rest) < 2 {
			return nil, errors.New("csdftracechkcmd.NewParseOptionsFunc: want a diagram and at least one trace TSV")
		}

		diagram, err := tools.ValidateArgsAsFilePath(rest[:1], inout)
		if err != nil {
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		// Traces never come from standard input: the diagram may be reading it,
		// and two traces could not be told apart in one stream anyway.
		traces, err := tools.ReadFileInputs(rest[1:])
		if err != nil {
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: validate trace arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Match: m, Diagram: diagram, Traces: traces}, nil
	}
}
