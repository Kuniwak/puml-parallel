package csdfdecoratecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/csdf/decorate"
	"github.com/Kuniwak/puml-parallel/v6/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Bytes  []byte
	Words  decorate.Words
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfdecorate", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfdecorate [options] [file.puml]

Prints a Composable State Diagram with the label of each edge written for
people in PlantUML's Creole, for drawing: a line each for the event, the
condition, the change and the variables kept, one conjunct to a line, and
each variable and event field in colour, followed by where it comes from
(before or after the step, or the event) when its notation tells: $x,
x', or a field of the event. Every line stays where it was, so a drawing
of the output maps to the diagram as a drawing of the diagram does.
A file argument, a "-" argument, and standard input are all equivalent.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfdecorate -lang ja diagram_ja.puml | plantuml -pipe -tsvg > diagram.svg
  $ csdfreplweb -puml-exec "sh -c 'csdfdecorate diagram_en.puml | plantuml -pipe -tsvg'" diagram_formal.puml > diagram.html
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		lang := flags.String("lang", "en", "the language of the headings and origins: en or ja")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfdecoratecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfdecoratecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}
		words, ok := decorate.WordsOf(*lang)
		if !ok {
			return nil, fmt.Errorf("csdfdecoratecmd.NewParseOptionsFunc: -lang %q: want en or ja", *lang)
		}

		bs, err := tools.ValidateArgsAsFilePath(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfdecoratecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Bytes: bs, Words: words}, nil
	}
}
