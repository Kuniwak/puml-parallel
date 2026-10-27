package csdfrepldcmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation/proto"
	"github.com/Kuniwak/puml-parallel/v3/tools"
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

		sock := tools.ResolveSocketPath(opts.Sock, inout.Env)
		check := proto.EnvironmentCheck(proto.AnyEnvironment)
		suggesters := animation.SuggesterFactory(animation.NoSuggesters)
		if opts.Checker.Asks() {
			check = opts.Checker.Agrees
			suggesters = opts.Checker.Suggesters(bmc.RunCsdfbmc)
		}
		service := proto.NewService(version.Version, opts.Common.Debug(), check, suggesters)

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(interrupts)

		return serve(sock, service, inout, interrupts)
	}
}
