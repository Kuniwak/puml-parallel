package obligationirccmd

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
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

		var ir obligationir.ObligationIR
		if err := json.Unmarshal(opts.Bytes, &ir); err != nil {
			return fmt.Errorf("obligationirccmd.NewMainFunc: invalid obligation IR JSON: %w", err)
		}

		if err := target.Compile(inout.Stdout, ir, opts.Target); err != nil {
			return fmt.Errorf("obligationirccmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
