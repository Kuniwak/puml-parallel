package csdfreplcmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
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

		replay, checker, err := loadReplay(opts)
		if err != nil {
			return err
		}
		run := runOptions{Replay: replay}
		if checker.Asks() {
			if opts.DiagramJSON != "" {
				run.Suggester = &bmc.Suggester{Diagram: opts.DiagramJSON, Checker: checker}
			} else {
				run.SuggesterForSource = func(source []byte) (animation.Suggester, func(), error) {
					return bmc.ForSource(source, checker)
				}
			}
		}
		return runWithSolver(opts.File, inout, interrupts, csdf.SolveJSON, run)
	}
}

// loadReplay reads the finding -replay and -finding name, if any, and the
// checker the value prompts ask: with a report, the one walking the
// environment the report was made in (bmc.Checker.Reproduce), so the
// suggestions after the replayed path are what csdfbmc saw there.
func loadReplay(opts *Options) (*animation.Replay, bmc.Checker, error) {
	if opts.Replay == "" {
		return nil, opts.Checker, nil
	}
	bs, err := os.ReadFile(opts.Replay)
	if err != nil {
		return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: cannot read the replay file: %w: %q", err, opts.Replay)
	}
	findings, err := animation.LoadReplays(bs)
	if err != nil {
		return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: %w", err)
	}
	if len(findings) == 0 {
		return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: %q records no finding", opts.Replay)
	}
	if opts.Finding < 0 || opts.Finding >= len(findings) {
		return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: no finding %d; %q records %d (0 to %d)", opts.Finding, opts.Replay, len(findings), len(findings)-1)
	}
	checker := opts.Checker
	if checker.Asks() {
		env, err := animation.LoadEnvironment(bs)
		if err != nil {
			return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: %w", err)
		}
		if checker, err = checker.Reproduce(env); err != nil {
			return nil, bmc.Checker{}, fmt.Errorf("csdfreplcmd.loadReplay: %q: %w", opts.Replay, err)
		}
	}
	return &findings[opts.Finding].Replay, checker, nil
}
