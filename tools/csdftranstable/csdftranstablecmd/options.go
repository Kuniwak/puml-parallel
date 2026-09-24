package csdftranstablecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common   *tools.CommonOptions
	Post     bool
	ExprMode ExprMode
	Bytes    []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdftranstable", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdftranstable [options] [diagram.puml|diagram.png|-]

Writes the state transition table of a Composable State Diagram as TSV: a row
per reachable state, a column per event, and in every cell what may happen when
the environment offers that event in that state. It is there to make refusals
visible, so that "this state should not refuse that" is easy to notice.

The columns are "state" (the state ID), "name" (the state name), the visible
events, and "[*]" when a reachable state has an end edge. States and events
come in the order a breadth-first walk from the start state meets them.

A cell is read in the stable-failures sense, and holds one outcome per line:

  [c] → s   the event is accepted and the diagram goes to s, when c holds
  [c] ×     the event is refused, when c holds

A line without [c] happens unconditionally. The diagram may first take tau
edges, which the environment cannot see, so an outcome may happen after some;
its condition then conjoins every guard along the way, in order, and a later
guard reads the values the postconditions before it left. The diagram refuses
an event where it is stable (no tau guard holds) and no guard for the event
holds. Every line holds on its own: a cell is the set of what may happen.

The guards are natural language and are never evaluated; a postcondition is
taken to admit some next values, and whether the guards cover every case is
left to the reader. A reachable tau cycle may make the diagram diverge, which a
table of refusals cannot show, so such a diagram is refused. Unreachable states
have no row, and their IDs are written to standard error.

A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdftranstable examples/valid/vending_machine.puml
  $ csdftranstable -post -expr-mode logical examples/valid/vending_machine.puml
  $ csdfcomp tree.json | csdftranstable -
`)
		}

		var post bool
		flags.BoolVar(&post, "post", false, "write the postconditions along the path of each outcome, after a slash")

		var exprMode string
		flags.StringVar(&exprMode, "expr-mode", string(ExprModeNatural), "how conditions are spelled: natural (and, not, then) or logical (∧, ¬, ⨾)")

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		mode, err := ParseExprMode(exprMode)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Post: post, ExprMode: mode, Bytes: bs}, nil
	}
}
