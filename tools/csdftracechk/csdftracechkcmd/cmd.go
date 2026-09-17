package csdftracechkcmd

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
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

		diagram, err := csdf.ParseBytes(opts.Diagram)
		if err != nil {
			return fmt.Errorf("csdftracechkcmd.NewMainFunc: %w", err)
		}

		// Every trace is reported, and the exit status is the verdict on top of
		// the reports: a run that fails still has to say where each trace failed.
		rejected := 0
		for _, trace := range opts.Traces {
			result := tracechk.Run(opts.Match, diagram, trace.Name, trace.Events)
			if !result.Accepted() {
				rejected++
			}
			if err := tracechk.WriteMarkdown(inout.Stdout, diagram, result); err != nil {
				return fmt.Errorf("csdftracechkcmd.NewMainFunc: %w", err)
			}
		}
		if rejected > 0 {
			return errors.New("csdftracechkcmd.NewMainFunc: some trace is not a trace of the diagram even with every guard true")
		}
		return nil
	}
}
