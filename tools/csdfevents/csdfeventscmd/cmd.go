package csdfeventscmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
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

		var events []string
		var err error
		if opts.OnlyCommon {
			events, err = collectCommonEvents(opts.Files)
		} else {
			events, err = collectAllEvents(opts.Files)
		}
		if err != nil {
			return err
		}

		for _, event := range events {
			fmt.Fprintln(inout.Stdout, event)
		}
		return nil
	}
}
