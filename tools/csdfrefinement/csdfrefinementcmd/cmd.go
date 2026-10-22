package csdfrefinementcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
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

		spec, err := coreio.ReadBytes(opts.Spec)
		if err != nil {
			return fmt.Errorf("csdfrefinementcmd.NewMainFunc: spec: %w", err)
		}
		impl, err := coreio.ReadBytes(opts.Impl)
		if err != nil {
			return fmt.Errorf("csdfrefinementcmd.NewMainFunc: impl: %w", err)
		}

		// Compile the proof obligation to the selected target and exit 0. Whether
		// the refinement holds depends on the environment - the atoms, functions
		// and types - which this tool does not interpret, so it never decides the
		// verdict via exit status.
		if err := target.CompileDiagrams(inout.Stdout, opts.Mode, spec, impl, opts.Target); err != nil {
			return fmt.Errorf("csdfrefinementcmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
