package csdftracechkcmd

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"

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
			fmt.Fprintf(w, `Usage: csdftracechk [options] <diagram.puml|diagram.png|-> [trace.tsv ...]

Checks whether each trace is a trace of the Composable State Diagram in the
stable-failures sense, and writes a Markdown report to standard output. A trace
is a TSV whose header is the single column "event" and whose every other row is
one visible event; it is read as CSV with a tab delimiter, so a field may be
quoted to hold a tab or a newline, and blank lines are skipped. The internal
event "tau" cannot appear.

The reading is the one under which an environment offering the events one at a
time is guaranteed to have each accepted: after every prefix, no stable state
the diagram may have reached by performing it refuses the next event. So a
nondeterministic branch that gets stuck rejects the trace, where the plain
traces reading would have let the other branch carry it.

The check first takes every guard as true. If some stable state reachable after
a prefix has no edge for the next event, the report says which prefix, which
states, and along which paths that state is reached, together with the
condition under which the refusal is not real after all (every such path is
infeasible), and the exit status is 1. Otherwise the report lists, for every
event and every path performing the prefix before it (tau steps included), the
guards and postconditions along the path and the obligation

  ∀ x0 ... xi. post_0(x0) ∧ guard_1(x0) ∧ post_1(x0, x1) ∧ ... → unstable(xi) ∨ enabled(xi)

that the state reached is unstable (some tau edge is enabled) or can perform
the event (some edge for it has its guard true and its post satisfiable),
followed by a prompt asking a reader (a person or an LLM) to decide them. The
predicates are natural language, so this tool never decides that itself; the
exit status is 0 when every trace passed the all-guards-true check. A path
never revisits a state within one run of tau edges, so a tau cycle yields
finitely many paths and the ones going round it are not listed; divergence is
csdflivelockfree's business.

Events are free-form text and their notation is not fixed, so the way a trace
event is looked up in the diagram is one of two simple rules chosen by -match:

  exact   the identical text (default)
  prefix  the trace event is a prefix of the diagram event, so a trace whose
          parameters were stripped ("insert") matches the label "insert(coin)"

Anything richer is best done to the trace beforehand, e.g. with sed or qhs. The
diagram may be "-" for standard input; traces must be files. Too many traces
for the command line can be listed in a TSV given by -traces, whose header is
the single column "path" and whose every other row is the path of a trace TSV
(relative to the working directory); it is read as a trace TSV is, is added to
the trace arguments, and may be "-" for standard input unless the diagram is.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdftracechk examples/valid/vending_machine.puml trace.tsv
  $ csdftracechk -match prefix examples/valid/vending_machine.puml trace1.tsv trace2.tsv
  $ csdfparallel -sync 'insert(coin)' a.puml b.puml | csdftracechk - trace.tsv
  $ (echo path; find traces -name '*.tsv') | csdftracechk -traces - examples/valid/vending_machine.puml
`)
		}

		var match string
		flags.StringVar(&match, "match", string(MatchNameExact), "how a trace event is looked up in the diagram: exact|prefix")

		var traceList string
		flags.StringVar(&traceList, "traces", "", "a TSV listing trace TSV paths under the header \"path\", read in addition to the trace arguments; \"-\" for standard input")

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
		if len(rest) < 1 {
			return nil, errors.New("csdftracechkcmd.NewParseOptionsFunc: want a diagram")
		}
		if rest[0] == "-" && traceList == "-" {
			return nil, errors.New("csdftracechkcmd.NewParseOptionsFunc: the diagram and -traces cannot both be \"-\"")
		}

		diagram, err := tools.ValidateArgsAsFilePath(rest[:1], inout)
		if err != nil {
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		// Traces themselves never come from standard input: two traces could not
		// be told apart in one stream. Only the list of their paths may.
		tracePaths := rest[1:]
		if traceList != "" {
			listed, err := readTraceList(traceList, inout)
			if err != nil {
				return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: read -traces failed: %w", err)
			}
			tracePaths = append(tracePaths, listed...)
		}
		if len(tracePaths) == 0 {
			return nil, errors.New("csdftracechkcmd.NewParseOptionsFunc: want at least one trace TSV")
		}
		traces, err := tools.ReadFileInputs(tracePaths)
		if err != nil {
			return nil, fmt.Errorf("csdftracechkcmd.NewParseOptionsFunc: validate trace arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Match: m, Diagram: diagram, Traces: traces}, nil
	}
}

// TraceListHeader is the single column of a -traces TSV.
const TraceListHeader = "path"

func readTraceList(file string, inout *cli.ProcInout) ([]string, error) {
	bs, err := tools.ValidateArgsAsFilePath([]string{file}, inout)
	if err != nil {
		return nil, err
	}
	return ReadTraceList(bytes.NewReader(bs))
}

// ReadTraceList reads a TSV whose header is the single column "path" and whose
// every other row is the path of a trace TSV. It is read the way a trace TSV
// is: as CSV with a tab delimiter, skipping blank lines. A relative path is
// relative to the working directory, as a trace argument is.
func ReadTraceList(r io.Reader) ([]string, error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.FieldsPerRecord = 1

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csdftracechkcmd.ReadTraceList: %w", err)
	}
	if len(records) == 0 || records[0][0] != TraceListHeader {
		return nil, fmt.Errorf("csdftracechkcmd.ReadTraceList: want a header row %q", TraceListHeader)
	}

	paths := make([]string, 0, len(records)-1)
	for _, record := range records[1:] {
		paths = append(paths, record[0])
	}
	return paths, nil
}
