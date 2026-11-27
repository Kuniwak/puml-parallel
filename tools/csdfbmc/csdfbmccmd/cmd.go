package csdfbmccmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/Kuniwak/puml-parallel/v8/cli"
	"github.com/Kuniwak/puml-parallel/v8/csdfbmc"
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/version"
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

		python, err := csdfbmc.Interpreter(opts.Python, inout.Env)
		if err != nil {
			return &tools.ExitStatusError{Status: 2, Message: err.Error()}
		}
		dir, err := csdfbmc.Materialize(csdfbmc.CacheRoot(inout.Env))
		if err != nil {
			return fmt.Errorf("csdfbmccmd.NewMainFunc: %w", err)
		}

		module, args := csdfbmc.ModuleCheck, opts.Args
		if len(args) > 0 {
			if sub, ok := subcommands[args[0]]; ok {
				module, args = sub, args[1:]
			}
		}
		cmd := exec.Command(python, csdfbmc.ModuleArgs(dir, module, args...)...)
		cmd.Stdin = inout.Stdin
		cmd.Stdout = inout.Stdout
		cmd.Stderr = inout.Stderr
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		if err := runRelaying(cmd, signals); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				// The checker has said what it found or what went wrong.
				return &tools.ExitStatusError{Status: exit.ExitCode()}
			}
			return &tools.ExitStatusError{Status: 2, Message: fmt.Sprintf("cannot run %s: %v", python, err)}
		}
		return nil
	}
}

// subcommands are the modules a first argument names in place of the checker.
var subcommands = map[string]string{"next": csdfbmc.ModuleNext, "shapes": csdfbmc.ModuleShapes}

// runRelaying runs cmd and passes each signal of signals on to it until it
// ends: the checker stops on SIGINT or SIGTERM with the report of what it
// found, which a launcher killed in its place would never wait for.
func runRelaying(cmd *exec.Cmd, signals <-chan os.Signal) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case err := <-done:
			return err
		case s := <-signals:
			// Windows cannot send a signal to a process; ^C reaches the
			// checker from the console there.
			_ = cmd.Process.Signal(s)
		}
	}
}
