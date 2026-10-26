package csdfparallelcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/tools"
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

		diagrams, err := coreio.LoadAll(opts.Files)
		if err != nil {
			return fmt.Errorf("csdfparallelcmd.NewMainFunc: %w", err)
		}

		composite, err := core.ComposeAll(diagrams, opts.Sync)
		if err != nil {
			return fmt.Errorf("csdfparallelcmd.NewMainFunc: %w", err)
		}
		composite.Name = tools.GeneratedBy("csdfparallel", opts.Args)

		text, err := coreio.WriteCanonical(composite)
		if err != nil {
			return fmt.Errorf("csdfparallelcmd.NewMainFunc: %w", err)
		}
		fmt.Fprint(inout.Stdout, text)
		return nil
	}
}
