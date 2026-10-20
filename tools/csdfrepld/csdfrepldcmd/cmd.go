package csdfrepldcmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/tools"
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

		sock := tools.ResolveSocketPath(opts.Sock, inout.Env)
		service := proto.NewService(version.Version, opts.Common.Debug())
		if opts.Twins != "" {
			service.SetSuggesters(func(d *csdf.Diagram, diagramJSON string) (animation.Suggester, func(), error) {
				if diagramJSON != "" {
					return &bmc.Suggester{Diagram: diagramJSON, Twins: opts.Twins, Locals: opts.Locals}, func() {}, nil
				}
				return bmc.ForDiagram(d, opts.Twins, opts.Locals)
			})
		}

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(interrupts)

		return serve(sock, service, inout, interrupts)
	}
}
