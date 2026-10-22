package csdflivelockfreecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/Kuniwak/puml-parallel/pngsrc"
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

		source, err := pngsrc.Extract(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: reading PlantUML source: %w", err)
		}
		diagram, err := coreio.Read(source)
		if err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}

		// Compile the proof-obligation IR to the selected target and exit 0. Livelock
		// freedom depends on the environment - the atoms, functions and types - which
		// this tool does not interpret, so it never decides the verdict via exit status.
		ir, err := obligationir.BuildFormulaLivelockFree(diagram)
		if err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}
		if err := target.Compile(inout.Stdout, ir, opts.Target); err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
