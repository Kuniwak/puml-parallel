package csdfreplwebcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
)

// DefaultPumlExec is the PlantUML command -puml-exec runs unless told
// otherwise: plantuml on the PATH, reading the source on its standard input
// and writing SVG.
const DefaultPumlExec = "plantuml -pipe -tsvg"

type Options struct {
	Common *tools.CommonOptions
	File   string
	// PumlExec is the command, split into words, that reads PlantUML source
	// on its standard input and writes the SVG of it on its standard output.
	PumlExec []string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfreplweb", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfreplweb [options] <file.puml|file.png>

Writes a web page that explores a Composable State Diagram as csdfrepl
does, to the standard output. The page is one HTML file that needs nothing
but a browser: open it from disk or put it on any static web host. It draws
the diagram, highlights where the run is and what can happen next, takes
the values of each state in a form, shows the history and lets one jump
back to any step of it, and saves the path explored as a file csdfrepl
-replay walks. Its words are in English and Japanese.

The drawing is the SVG PlantUML renders of the diagram: csdfreplweb runs
the command -puml-exec gives, in the diagram's directory, with the source
on its standard input, and reads the SVG on its standard output. The
command is split into words as a shell splits them, quotes included, but
no shell runs it; give the options that make PlantUML read the standard
input and write SVG yourself (-pipe -tsvg).

A global diagram holding promotion directives (docs/PROMOTION.md) is
explored as its expansion, as csdfrepl explores it, each !include read
relative to the file; its drawing is the global diagram, without
highlights. The page suggests no values: csdfbmc does not run in a
browser.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfreplweb examples/valid/vending_machine.puml > vending_machine.html
  $ csdfreplweb -puml-exec 'java -jar "/opt/plant uml/plantuml.jar" -pipe -tsvg' diagram.puml > diagram.html
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		pumlExec := flags.String("puml-exec", DefaultPumlExec, "command that reads PlantUML source on its standard input and writes SVG on its standard output")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		argv, err := SplitCommandLine(*pumlExec)
		if err != nil {
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: -puml-exec: %w", err)
		}
		if flags.NArg() != 1 {
			return nil, errors.New("csdfreplwebcmd.NewParseOptionsFunc: exactly one CSDF file is required")
		}

		return &Options{Common: commonOpts, File: flags.Arg(0), PumlExec: argv}, nil
	}
}
