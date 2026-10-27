package csdfhidecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v3/version"
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

		diagram, err := coreio.ReadBytes(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfhidecmd.NewMainFunc: %w", err)
		}
		hidden, err := core.Hide(diagram, opts.Channels)
		if err != nil {
			return fmt.Errorf("csdfhidecmd.NewMainFunc: %w", err)
		}
		text, err := coreio.WriteCanonical(hidden)
		if err != nil {
			return fmt.Errorf("csdfhidecmd.NewMainFunc: %w", err)
		}
		fmt.Fprint(inout.Stdout, text)
		return nil
	}
}
