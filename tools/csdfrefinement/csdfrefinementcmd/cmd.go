package csdfrefinementcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
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

		spec, err := csdf.ParseBytes(opts.Spec)
		if err != nil {
			return fmt.Errorf("csdfrefinementcmd.NewMainFunc: spec: %w", err)
		}
		impl, err := csdf.ParseBytes(opts.Impl)
		if err != nil {
			return fmt.Errorf("csdfrefinementcmd.NewMainFunc: impl: %w", err)
		}

		// Compile the proof-obligation IR to the selected target and exit 0.
		// Whether the refinement holds depends on the natural-language predicates,
		// which this tool does not interpret, so it never decides the verdict via
		// exit status.
		ir := obligationir.BuildRefinement(opts.Mode, spec, impl)
		if err := target.CompileRefinement(inout.Stdout, ir, opts.Target); err != nil {
			return fmt.Errorf("csdfrefinementcmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
