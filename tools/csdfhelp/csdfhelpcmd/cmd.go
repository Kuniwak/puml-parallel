package csdfhelpcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/toolsdoc"
	"github.com/Kuniwak/puml-parallel/version"
)

// NewCommandFunc composes csdfhelp's own command function. Its registry entry
// and its main package both need it, so it is decided here once.
func NewCommandFunc() cli.CommandFunc {
	return tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
}

func NewMainFunc() cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}

		entries := toolsdoc.Select(Registry(), opts.ToolNames)

		if opts.Short {
			if err := toolsdoc.WriteSummaries(inout.Stdout, entries); err != nil {
				return fmt.Errorf("csdfhelpcmd.NewMainFunc: %w", err)
			}
			return nil
		}

		if err := toolsdoc.WriteMarkdown(inout.Stdout, inout.Env, entries); err != nil {
			return fmt.Errorf("csdfhelpcmd.NewMainFunc: %w", err)
		}
		return nil
	}
}
