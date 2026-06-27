package csdflivelockfreecmd

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
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

		diagram, err := csdf.ParseDiagram(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}

		// Emit the proof-obligation IR and exit 0. Livelock freedom depends on the
		// natural-language predicates, which this tool does not interpret, so it
		// never decides the verdict via exit status.
		if err := json.NewEncoder(inout.Stdout).Encode(obligationir.BuildObligationIR(diagram)); err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
