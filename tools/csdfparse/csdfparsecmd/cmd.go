package csdfparsecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v4/cli"
	"github.com/Kuniwak/puml-parallel/v4/csdf"
	"github.com/Kuniwak/puml-parallel/v4/csdf/core"
	"github.com/Kuniwak/puml-parallel/v4/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v4/pngsrc"
	"github.com/Kuniwak/puml-parallel/v4/version"
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

		source, err := pngsrc.Extract(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfparsecmd.NewMainFunc: reading PlantUML source: %w", err)
		}
		tree, err := csdf.Parse(source)
		if err != nil {
			return fmt.Errorf("csdfparsecmd.NewMainFunc: %w", err)
		}
		d, err := coreio.Lower(tree)
		if err != nil {
			return fmt.Errorf("csdfparsecmd.NewMainFunc: %w", err)
		}
		raw, err := core.MarshalDiagram(d)
		if err != nil {
			return fmt.Errorf("csdfparsecmd.NewMainFunc: %w", err)
		}
		fmt.Fprintln(inout.Stdout, string(raw))
		return nil
	}
}
