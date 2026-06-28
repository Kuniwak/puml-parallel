package obligationirccmd

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
	irjson "github.com/Kuniwak/puml-parallel/csdf/obligationir/json"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/lean"
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

		var ir obligationir.ObligationIR
		if err := json.Unmarshal(opts.Bytes, &ir); err != nil {
			return fmt.Errorf("obligationirccmd.NewMainFunc: invalid obligation IR JSON: %w", err)
		}

		var compileErr error
		switch opts.Target {
		case TargetIsabelle:
			compileErr = isabelle.Compile(inout.Stdout, ir)
		case TargetLean:
			compileErr = lean.Compile(inout.Stdout, ir)
		default: // TargetIRJSON
			compileErr = irjson.Compile(inout.Stdout, ir)
		}
		if compileErr != nil {
			return fmt.Errorf("obligationirccmd.NewMainFunc: %w", compileErr)
		}
		return nil
	}
}
