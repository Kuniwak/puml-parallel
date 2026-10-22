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

		diagrams := make([]*core.Diagram, len(opts.Files))
		for i, f := range opts.Files {
			d, err := coreio.Load(f)
			if err != nil {
				return fmt.Errorf("csdfparallelcmd.NewMainFunc: cannot parse diagrams: %w", err)
			}
			diagrams[i] = d
		}

		// Left to right, so the composite names each field as the first
		// diagram does.
		composite := diagrams[0]
		for _, d := range diagrams[1:] {
			var err error
			if composite, err = core.Compose(composite, d, opts.Sync); err != nil {
				return fmt.Errorf("csdfparallelcmd.NewMainFunc: %w", err)
			}
			composite = core.Reachable(composite)
		}
		composite.Name = tools.GeneratedBy("csdfparallel", opts.Args)

		canonical, err := coreio.Canonical(composite)
		if err != nil {
			return fmt.Errorf("csdfparallelcmd.NewMainFunc: %w", err)
		}
		text, err := coreio.Write(canonical)
		if err != nil {
			return fmt.Errorf("csdfparallelcmd.NewMainFunc: %w", err)
		}
		fmt.Fprint(inout.Stdout, text)
		return nil
	}
}
