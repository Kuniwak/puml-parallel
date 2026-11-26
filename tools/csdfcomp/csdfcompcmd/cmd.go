package csdfcompcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/csdf"
	"github.com/Kuniwak/puml-parallel/v6/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/version"
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

		expr, err := csdf.ParseExpr(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfcompcmd.NewMainFunc: %w", err)
		}

		composite, err := coreio.ComposeTree(expr, coreio.FileLoader(opts.BaseDir))
		if err != nil {
			return fmt.Errorf("csdfcompcmd.NewMainFunc: %w", err)
		}
		composite.Name = tools.GeneratedBy("csdfcomp", opts.Args)

		canonical, err := coreio.Canonical(composite)
		if err != nil {
			return fmt.Errorf("csdfcompcmd.NewMainFunc: %w", err)
		}
		text, err := coreio.Write(canonical)
		if err != nil {
			return fmt.Errorf("csdfcompcmd.NewMainFunc: %w", err)
		}
		fmt.Fprint(inout.Stdout, text)
		return nil
	}
}
