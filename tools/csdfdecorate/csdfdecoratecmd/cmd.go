package csdfdecoratecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v8/cli"
	"github.com/Kuniwak/puml-parallel/v8/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v8/csdf/decorate"
	"github.com/Kuniwak/puml-parallel/v8/version"
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
		d, err := coreio.ReadBytes(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfdecoratecmd.NewMainFunc: %w", err)
		}
		out, err := decorate.Decorate(string(opts.Bytes), d, opts.Words)
		if err != nil {
			return fmt.Errorf("csdfdecoratecmd.NewMainFunc: %w", err)
		}
		fmt.Fprint(inout.Stdout, out)
		return nil
	}
}
