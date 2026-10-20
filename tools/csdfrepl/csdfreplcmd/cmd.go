package csdfreplcmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
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

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)

		var replay *animation.Replay
		if opts.Replay != "" {
			bs, err := os.ReadFile(opts.Replay)
			if err != nil {
				return fmt.Errorf("csdfreplcmd.NewMainFunc: cannot read the replay file: %w: %q", err, opts.Replay)
			}
			findings, err := animation.LoadReplays(bs)
			if err != nil {
				return fmt.Errorf("csdfreplcmd.NewMainFunc: %w", err)
			}
			if len(findings) == 0 {
				return fmt.Errorf("csdfreplcmd.NewMainFunc: %q records no finding", opts.Replay)
			}
			if opts.Finding < 0 || opts.Finding >= len(findings) {
				return fmt.Errorf("csdfreplcmd.NewMainFunc: no finding %d; %q records %d (0 to %d)", opts.Finding, opts.Replay, len(findings), len(findings)-1)
			}
			replay = &findings[opts.Finding].Replay
		}
		return runWithSolver(opts.File, inout, interrupts, csdf.SolveJSON, replay)
	}
}
