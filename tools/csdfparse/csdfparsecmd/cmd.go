package csdfparsecmd

import (
	"encoding/json"
	"fmt"
	"io"

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

		var diagram *core.Diagram
		if opts.File == "" || opts.File == "-" {
			content, err := io.ReadAll(inout.Stdin)
			if err != nil {
				return fmt.Errorf("reading from stdin: %w", err)
			}
			diagram, err = csdf.ParseDiagram(content)
			if err != nil {
				return err
			}
		} else {
			loaded, err := csdf.LoadDiagram(opts.File)
			if err != nil {
				return err
			}
			diagram = loaded
		}

		if err := json.NewEncoder(inout.Stdout).Encode(diagram); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
		return nil
	}
}
