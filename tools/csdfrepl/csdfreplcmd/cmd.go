package csdfreplcmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation/bmc"
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

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)

		replay, checker, err := loadReplay(opts)
		if err != nil {
			return err
		}
		run := runOptions{Replay: replay, Suggesters: animation.NoSuggesters}
		if checker.Asks() {
			run.Suggesters = checker.Suggesters(bmc.RunCsdfbmc)
		}
		return runREPL(opts.File, inout, interrupts, run)
	}
}

// loadReplay reads the finding -replay and -finding name, if any, and the
// checker the value prompts ask (bmc.LoadFinding), so the suggestions after
// the replayed path are what csdfbmc saw there.
func loadReplay(opts *Options) (*animation.Replay, bmc.Checker, error) {
	if opts.Replay == "" {
		return nil, opts.Checker, nil
	}
	bs, err := os.ReadFile(opts.Replay)
	if err != nil {
		return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: cannot read the replay file: %w: %q", err, opts.Replay)
	}
	finding, checker, err := bmc.LoadFinding(bs, opts.Finding, opts.Checker)
	if err != nil {
		return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: %q: %w", opts.Replay, err)
	}
	return &finding.Replay, checker, nil
}
