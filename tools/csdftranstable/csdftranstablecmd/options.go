package csdftranstablecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/logic"
	"github.com/Kuniwak/puml-parallel/v3/csdf/transtable"
	"github.com/Kuniwak/puml-parallel/v3/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// Notation spells the conditions when Output spells them.
	Notation   logic.Notation
	Conditions transtable.Conditions
	Output     transtable.Output
	Bytes      []byte
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
Every line holds on its own: a cell is the set of what may happen, and a line
that comes out the same as one before it is written once.

C is a formula of first-order logic: the guards and posts along its way, the
formulas their clauses are (docs/SYNTAX.md), each atom quoted as a JSON string
and applied to its holes. The variables are

  v      the state variable v of the row's state
  k      the field k the column's event receives, as the environment offers it
  v'     the variable v of the state an accepted event leads to
  v@i    the variable v after the i-th tau step

The diagram may first take tau edges, which the environment cannot see, so C
conjoins every guard and post along the way, in order, and binds the values in
between. The diagram refuses an event where it is stable and cannot perform it:
no tau edge is enabled and no edge for the event is. Every post is taken to
admit some values after its step, so an edge is enabled exactly when its guard
holds, of the values and, for the event, of the fields offered.

With -conditions enabling, C leaves out every postcondition whose values
nothing kept after it reads: the one of the accepted event always, and one of
a tau step unless a later guard reads the values after that step. Since every
post admits some values, C holds of the fields and the values exactly where
the full condition holds for some v'; it says what the outcome is enabled by, and no
longer what the values after are.

Negation binds tightest; a conjunction inside a disjunction, or the other way
round, is parenthesised; an implication binds loosest; and a quantifier
reaches to the end of what it is in, so it is parenthesised exactly when
something follows it.

The atoms are never evaluated,
and whether the guards cover every case is left to the reader. A reachable tau
cycle may make the diagram diverge, which a table of refusals cannot show, so
such a diagram is refused, and so is one naming a state it never declares.
Unreachable states have no row, and their IDs are written to standard error.
An event spelled "state", "name" or "[*]" would be read as that column of the
TSV, so it is an error there.

With -format json, the table is written as JSON instead, for a program to read:

  {"columns": [{"kind": "event", "event": E} | {"kind": "termination"}, ...],
   "rows": [{"state": ID, "name": NAME, "cells": [[OUTCOME, ...], ...]}, ...],
   "unreachable": [ID, ...]}

with a cell per column in the same order. An OUTCOME is
{"result": "goto", "state": ID, "condition": C}, or "terminate" or "refuse"
without "state", and C is a syntax tree: {"op": OP, ...}, where OP is true,
false, atom (with "name", "quoted" and "terms", each {"op": "var", "name": V}
or an index, field or apply term), not, and, or, implies (with
"operands"), exists or forall (with "vars" and "operands"), over the variables
above. Being no TSV, it has no column to clash with, and -expr-mode does
not apply.

A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdftranstable examples/valid/vending_machine.puml
  $ csdftranstable -expr-mode logical examples/valid/vending_machine.puml
  $ csdftranstable -conditions enabling examples/valid/vending_machine.puml
  $ csdftranstable -format json examples/valid/vending_machine.puml
  $ csdfcomp tree.json | csdftranstable -
`)
		}

		var exprMode string
		flags.StringVar(&exprMode, "expr-mode", transtable.NotationNatural, "how conditions are spelled: natural (and, not, exists) or logical (∧, ¬, ∃)")

		var conditions string
		flags.StringVar(&conditions, "conditions", transtable.ConditionsNameFull, "how much conditions keep: full (every guard and postcondition) or enabling (without the postconditions nothing reads)")

		var format string
		flags.StringVar(&format, "format", transtable.OutputNameTSV, "what the table is written as: tsv (for a reader) or json (for a program)")

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

		notation, err := transtable.ParseNotation(exprMode)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: %w", err)
		}

		output, err := transtable.ParseOutput(format)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: %w", err)
		}
		if !output.SpellsConditions() && tools.FlagGiven(flags, "expr-mode") {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: %w", fmt.Errorf("-expr-mode has no effect with -format %s, where a condition is a syntax tree", format))
		}

		conds, err := transtable.ParseConditions(conditions)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: %w", err)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdftranstablecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Notation: notation, Conditions: conds, Output: output, Bytes: bs}, nil
	}
}
