package csdfsortcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v3/pngsrc"
	"github.com/Kuniwak/puml-parallel/v3/version"
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

		text, err := canonical(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdfsortcmd.NewMainFunc: %w", err)
		}
		fmt.Fprint(inout.Stdout, text)
		return nil
	}
}

// canonical is the diagram in raw in its canonical order: a global diagram
// as written, reading none of the files it includes, any other as
// coreio.WriteCanonical writes it.
func canonical(raw []byte) (string, error) {
	src, err := pngsrc.Extract(raw)
	if err != nil {
		return "", fmt.Errorf("reading PlantUML source: %w", err)
	}
	if csdf.HoldsPromotionDirectives(src) {
		prom, err := coreio.ReadPromotionAsWritten(src)
		if err != nil {
			return "", err
		}
		return coreio.WritePromotion(prom)
	}
	diagram, err := coreio.ReadBytes(raw)
	if err != nil {
		return "", err
	}
	return coreio.WriteCanonical(diagram)
}
