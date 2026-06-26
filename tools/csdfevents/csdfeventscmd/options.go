package csdfeventscmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common     *tools.CommonOptions
	OnlyCommon bool
	Files      []string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfevents", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfevents [options] <file1.puml> [file2.puml] ...

Prints the events used across one or more Composable State Diagrams.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfevents a.puml
  $ csdfevents -only-common a.puml b.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		onlyCommon := flags.Bool("only-common", false, "show only common events across all files")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfeventscmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfeventscmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		files := flags.Args()
		if len(files) < 1 {
			return nil, fmt.Errorf("csdfeventscmd.NewParseOptionsFunc: too few arguments")
		}
		if *onlyCommon && len(files) < 2 {
			return nil, fmt.Errorf("csdfeventscmd.NewParseOptionsFunc: -only-common requires at least 2 files")
		}

		return &Options{
			Common:     commonOpts,
			OnlyCommon: *onlyCommon,
			Files:      files,
		}, nil
	}
}
