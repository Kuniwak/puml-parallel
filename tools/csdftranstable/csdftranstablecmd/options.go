package csdftranstablecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// ExprMode names the notation, one transtable.ParseNotation knows.
	ExprMode string
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

  [C] → s   the event may be accepted, and the diagram be in s
  [C] ×     the event may be refused

each when its condition C holds; a line without [C] happens unconditionally.
Every line holds on its own: a cell is the set of what may happen.

C is a formula of first-order logic whose atoms are the guards and
postconditions of the diagram, quoted as JSON strings and applied to what they
read: a guard to the parameters of its event and the values of the state
variables before its step, a postcondition to those and the values after (an
end edge reads the values only). The variables are

  x    the values of the row's state
  c    the parameters of the column's event, as the environment offers them
  x'   the values of the state an accepted event leads to
  ci   the parameters of the event of the i-th tau step
  xi   the values after the i-th tau step

The diagram may first take tau edges, which the environment cannot see, so C
conjoins every guard and postcondition along the way, in order, and binds the
values in between and the parameters of the hidden events. The parameters are
never parsed out of an event: a variable stands for them all, whatever they
are. The diagram refuses an event where it is stable and cannot perform it:
no tau edge is enabled and no edge for the event is. Every postcondition is
taken to admit some values after its step, so an edge is enabled exactly when
its guard holds for some parameters of a tau edge, or for those offered.

The guards and postconditions are natural language and are never evaluated,
and whether the guards cover every case is left to the reader. A reachable tau
cycle may make the diagram diverge, which a table of refusals cannot show, so
such a diagram is refused. Unreachable states have no row, and their IDs are
written to standard error.

A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdftranstable examples/valid/vending_machine.puml
  $ csdftranstable -expr-mode logical examples/valid/vending_machine.puml
  $ csdfcomp tree.json | csdftranstable -
`)
		}

		var exprMode string
		flags.StringVar(&exprMode, "expr-mode", transtable.NotationNatural, "how conditions are spelled: natural (and, not, exists) or logical (∧, ¬, ∃)")

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

		if _, err := transtable.ParseNotation(exprMode); err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, ExprMode: exprMode, Bytes: bs}, nil
	}
}
