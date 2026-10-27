package csdfbmccmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdfbmc"
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

		python, err := interpreter(opts, inout.Env)
		if err != nil {
			return &tools.ExitStatusError{Status: 2, Message: err.Error()}
		}
		dir, err := csdfbmc.Materialize(cacheRoot(inout.Env))
		if err != nil {
			return fmt.Errorf("csdfbmccmd.NewMainFunc: %w", err)
		}

		module, args := csdfbmc.ModuleCheck, opts.Args
		if len(args) > 0 && args[0] == "next" {
			module, args = csdfbmc.ModuleNext, args[1:]
		}
		cmd := exec.Command(python, csdfbmc.ModuleArgs(dir, module, args...)...)
		cmd.Stdin = inout.Stdin
		cmd.Stdout = inout.Stdout
		cmd.Stderr = inout.Stderr
		if err := cmd.Run(); err != nil {
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

// interpreter is the python3 to run: the -python option, then $CSDFBMC_PYTHON,
// then the first python3 on the PATH.
func interpreter(opts *Options, env cli.EnvFunc) (string, error) {
	named := opts.Python
	if named == "" {
		named = env("CSDFBMC_PYTHON")
	}
	if named != "" {
		if _, err := os.Stat(named); err != nil {
			return "", fmt.Errorf("the interpreter %s cannot be used: %v", named, err)
		}
		return named, nil
	}
	path, err := exec.LookPath("python3")
	if err != nil {
		return "", fmt.Errorf("python3 is not on the PATH; install Python 3.11 or later, or name an interpreter with -python or $CSDFBMC_PYTHON")
	}
	return path, nil
}

// cacheRoot is where the sources are unpacked: $CSDFBMC_CACHE, else the user
// cache directory, else a temporary directory.
func cacheRoot(env cli.EnvFunc) string {
	if root := env("CSDFBMC_CACHE"); root != "" {
		return root
	}
	if root, err := os.UserCacheDir(); err == nil {
		return root
	}
	return os.TempDir()
}
