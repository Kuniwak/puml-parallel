package csdflivelockfreecmd

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/version"
)

// ErrLivelockDetected is returned when the diagram is not livelock free. The CLI
// layer turns it into a non-zero exit status; the witness is printed to stdout.
var ErrLivelockDetected = errors.New("livelock detected")

func NewMainFunc() cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}

		diagram, err := csdf.ParseDiagram(opts.Bytes)
		if err != nil {
			return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", err)
		}

		witness, ok := csdf.CheckLivelockFree(diagram)
		if ok {
			fmt.Fprintln(inout.Stdout, "livelock free")
			return nil
		}

		fmt.Fprint(inout.Stdout, csdf.RenderLivelock(witness))
		return fmt.Errorf("csdflivelockfreecmd.NewMainFunc: %w", ErrLivelockDetected)
	}
}
