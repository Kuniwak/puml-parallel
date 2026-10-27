package csdflivelockfreecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligationir/target"
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
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}
		// Compile the proof obligation to the selected target and exit 0. Livelock
		// freedom depends on the environment - the atoms, functions and types - which
		// this tool does not interpret, so it never decides the verdict via exit status.
		if err := target.CompileDiagram(inout.Stdout, diagram, opts.Target); err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
