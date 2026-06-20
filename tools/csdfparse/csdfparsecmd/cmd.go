package csdfparsecmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
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

		input, err := io.ReadAll(inout.Stdin)
		if err != nil {
			return fmt.Errorf("reading from stdin: %w", err)
		}

		source, err := pngsrc.Extract(input)
		if err != nil {
			return fmt.Errorf("reading PlantUML source: %w", err)
		}

		diagram, err := core.NewParser(source).Parse()
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}

		if err := json.NewEncoder(inout.Stdout).Encode(diagram); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
		return nil
	}
}
