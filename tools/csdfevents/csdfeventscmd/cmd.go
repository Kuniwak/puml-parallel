package csdfeventscmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
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

		diagrams := make([]core.Diagram, 0, len(opts.Files))
		for _, file := range opts.Files {
			diagram, err := csdf.LoadDiagram(file)
			if err != nil {
				return err
			}
			diagrams = append(diagrams, *diagram)
		}

		var events []string
		if opts.OnlyCommon {
			events = csdf.CommonEvents(diagrams)
		} else {
			events = csdf.AllEvents(diagrams)
		}

		for _, event := range events {
			fmt.Fprintln(inout.Stdout, event)
		}
		return nil
	}
}
