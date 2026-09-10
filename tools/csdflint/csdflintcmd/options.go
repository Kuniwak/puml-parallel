package csdflintcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// ListRules asks for the catalog of rules instead of a run of them.
	ListRules bool
	Inputs    []tools.FileInput
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdflint", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)

		var listRules bool
		flags.BoolVar(&listRules, "list-rules", false, "list the rules and exit")

		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdflint [options] [file.puml|file.png ...]

Checks Composable State Diagrams and writes what it finds as a TSV table with
the columns file, start_line, end_line, rule, severity and message.
No file argument, and a "-" argument, both mean standard input.

A message is written to be read by a person and by an AI reviewing a diagram it
wrote itself, so it says what would settle the question, not only what looks
wrong. The severities are:

  ERROR          definitely wrong.
  WARN           probably not what the author meant.
  HINT           suspicious, but may well be intended.
  STYLE_PROBLEM  about how the source is written, not about what it means.

Exits 1 when anything is an ERROR, and 0 otherwise, so a HINT never fails a
build. Run csdflint -list-rules to see every rule.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdflint path/to/file.puml
  $ csdflint path/to/*.puml
  $ csdflint < path/to/file.puml
  $ csdflint -list-rules
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdflintcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdflintcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		// The catalog is about the tool and not about any input, so it does not
		// wait on standard input.
		if listRules {
			return &Options{Common: commonOpts, ListRules: true}, nil
		}

		inputs, err := tools.ValidateArgsAsFileInputs(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdflintcmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Inputs: inputs}, nil
	}
}
