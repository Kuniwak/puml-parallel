package csdfhidecmd

import (
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

		diagram, err := csdf.ParseBytes(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfhidecmd.NewMainFunc: %w", err)
		}

		fmt.Fprint(inout.Stdout, csdf.Hide(diagram, opts.Events).String())
		return nil
	}
}
