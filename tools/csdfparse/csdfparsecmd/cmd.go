package csdfparsecmd

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
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
			return err
		}

		if err := json.NewEncoder(inout.Stdout).Encode(diagram); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
		return nil
	}
}
