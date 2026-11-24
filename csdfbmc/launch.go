package csdfbmc

import (
	"fmt"
	"os"
	"os/exec"
)

// Interpreter is the python3 to run: named (a -python option), then
// $CSDFBMC_PYTHON, then the first python3 on the PATH. env reads the
// environment.
func Interpreter(named string, env func(string) string) (string, error) {
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

// CacheRoot is where the sources are unpacked (Materialize): $CSDFBMC_CACHE,
// else the user cache directory, else a temporary directory.
func CacheRoot(env func(string) string) string {
	if root := env("CSDFBMC_CACHE"); root != "" {
		return root
	}
	if root, err := os.UserCacheDir(); err == nil {
		return root
	}
	return os.TempDir()
}
