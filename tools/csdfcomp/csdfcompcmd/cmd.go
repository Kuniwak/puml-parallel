package csdfcompcmd

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

		expr, err := csdf.ParseExpr(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfcompcmd.NewMainFunc: %w", err)
		}

		composite, err := csdf.ComposeTree(expr, csdf.NewFileDiagramLoader(opts.BaseDir))
		if err != nil {
			return fmt.Errorf("csdfcompcmd.NewMainFunc: %w", err)
		}

		fmt.Fprint(inout.Stdout, composite.String())
		return nil
	}
}
