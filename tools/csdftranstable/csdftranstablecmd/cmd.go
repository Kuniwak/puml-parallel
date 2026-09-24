package csdftranstablecmd

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
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
			return fmt.Errorf("csdftranstablecmd.NewMainFunc: %w", err)
		}

		table, err := transtable.Build(diagram, transtable.Options{Posts: opts.Post})
		if err != nil {
			return fmt.Errorf("csdftranstablecmd.NewMainFunc: %w", err)
		}

		if len(table.Unreachable) > 0 {
			ids := make([]string, 0, len(table.Unreachable))
			for _, id := range table.Unreachable {
				ids = append(ids, string(id))
			}
			fmt.Fprintf(inout.Stderr, "warning: unreachable states have no row: %s\n", strings.Join(ids, ", "))
		}

		format := transtable.Format{Notation: opts.ExprMode.Notation()}
		if err := transtable.WriteTSV(inout.Stdout, table, format); err != nil {
			return fmt.Errorf("csdftranstablecmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
