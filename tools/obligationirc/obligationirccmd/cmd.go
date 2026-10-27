package obligationirccmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligationir"
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

		// A field this release does not know is refused rather than dropped: it
		// would be an IR of another version, whose meaning it does not read.
		dec := json.NewDecoder(bytes.NewReader(opts.Bytes))
		dec.DisallowUnknownFields()
		var ir obligationir.LivelockFree
		if err := dec.Decode(&ir); err != nil {
			return fmt.Errorf("obligationirccmd.NewMainFunc: invalid obligation IR JSON: %w", err)
		}

		if err := target.Compile(inout.Stdout, ir, opts.Target); err != nil {
			return fmt.Errorf("obligationirccmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
