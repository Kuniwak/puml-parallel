package csdfslicecmd

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
)

type Options struct {
	Common  *tools.CommonOptions
	States  []string
	Context int
	Bytes   []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfslice", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfslice -states A,B[,...] [-context N] [file.puml|file.png]

Prints the part of a Composable State Diagram around the given states: the
states within N edges of any of them, the edges followed in either
direction, and the edges between those states, in the order and rendering
of csdfsort. Every edge that leaves the slice is drawn to a placeholder
state labelled "…", and so is the start edge when the initial state is
outside; the placeholders and those edges sit in a CSDF-IGNORE region, so
PlantUML draws them but no tool reads them. A slice does not mean what the
diagram means - what is reachable changes - so it is for rendering only;
a slice without its initial state has no start edge for the other tools.
A diagram that holds promotion directives is refused, as csdfsort refuses
it. A file argument, a "-" argument, and standard input are all
equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfslice -states vmCoinsInserted path/to/file.puml
  $ csdfslice -states vmIdle,vmDropping -context 0 < path/to/file.puml
  $ csdfslice -states vmIdle path/to/file.puml | plantuml -pipe -tpng > slice.png
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		statesFlag := flags.String("states", "", "comma-separated list of the IDs of the states to slice around (required)")
		contextFlag := flags.Int("context", 1, "how many edges away from the given states a state may be and still be kept")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfslicecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfslicecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		states, err := parseStates(*statesFlag)
		if err != nil {
			return nil, fmt.Errorf("csdfslicecmd.NewParseOptionsFunc: -states: %w", err)
		}
		if *contextFlag < 0 {
			return nil, fmt.Errorf("csdfslicecmd.NewParseOptionsFunc: -context: want 0 or more, got %d", *contextFlag)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfslicecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, States: states, Context: *contextFlag, Bytes: bs}, nil
	}
}

// parseStates splits the comma-separated state IDs of -states, of which
// there has to be one at least, none empty.
func parseStates(s string) ([]string, error) {
	if s == "" {
		return nil, errors.New("want one state at least")
	}
	states := strings.Split(s, ",")
	for _, st := range states {
		if st == "" {
			return nil, fmt.Errorf("an empty state in %q", s)
		}
	}
	return states, nil
}
