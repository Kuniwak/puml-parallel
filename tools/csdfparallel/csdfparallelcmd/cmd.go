package csdfparallelcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/version"
)

func NewMainFunc() cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}

		diagrams, err := csdf.LoadDiagrams(opts.Files)
		if err != nil {
			return fmt.Errorf("csdfeventcmd.NewMainFunc: cannot parse diagrams: %w", err)
		}

		composite, err := csdf.ComposeParallel(diagrams, opts.Sync)
		if err != nil {
			return err
		}

		fmt.Fprint(inout.Stdout, composite.String())
		return nil
	}
}
