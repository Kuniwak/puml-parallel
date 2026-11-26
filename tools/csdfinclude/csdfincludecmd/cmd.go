package csdfincludecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v5/cli"
	"github.com/Kuniwak/puml-parallel/v5/csdf"
	"github.com/Kuniwak/puml-parallel/v5/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v5/version"
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

		resolved, err := coreio.ResolveFileIncludes(opts.Tree, opts.Base)
		if err != nil {
			return fmt.Errorf("csdfincludecmd.NewMainFunc: %w", err)
		}
		raw, err := csdf.MarshalSyntaxTree(resolved)
		if err != nil {
			return fmt.Errorf("csdfincludecmd.NewMainFunc: %w", err)
		}
		fmt.Fprintln(inout.Stdout, string(raw))
		return nil
	}
}
