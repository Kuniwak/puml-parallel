package csdfbmc

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Sources is the Python package itself, embedded so that the csdfbmc binary
// of a release carries the checker and needs nothing from a checkout.
//
//go:embed __init__.py __main__.py atoms.py checks.py cli.py cone.py environment.py explore.py formal.py model.py meaning.py next.py protocol.py python_glue.py reduce.py values.py
var Sources embed.FS

// Materialize writes the Python package under root, in a directory named by
// a hash of its contents, and returns the directory to put on PYTHONPATH.
// Writing happens once per content hash: a second call finds the directory
// and returns it, and two binaries with different sources never share one.
// The write is a rename of a finished temporary directory, so a reader never
// sees a half-written package.
func Materialize(root string) (string, error) {
	names, err := fs.Glob(Sources, "*.py")
	if err != nil {
		return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		bs, err := Sources.ReadFile(name)
		if err != nil {
			return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(bs))
		h.Write(bs)
	}
	dir := filepath.Join(root, "csdfbmc", hex.EncodeToString(h.Sum(nil))[:16])
	if _, err := os.Stat(filepath.Join(dir, "csdfbmc", "__init__.py")); err == nil {
		return dir, nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".tmp-")
	if err != nil {
		return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
	}
	pkg := filepath.Join(tmp, "csdfbmc")
	if err := os.Mkdir(pkg, 0o755); err != nil {
		return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
	}
	for _, name := range names {
		bs, _ := Sources.ReadFile(name)
		if err := os.WriteFile(filepath.Join(pkg, name), bs, 0o644); err != nil {
			os.RemoveAll(tmp)
			return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		os.RemoveAll(tmp)
		// Another process may have finished first; its copy is the same.
		if _, statErr := os.Stat(filepath.Join(dir, "csdfbmc", "__init__.py")); statErr == nil {
			return dir, nil
		}
		return "", fmt.Errorf("csdfbmc.Materialize: %w", err)
	}
	return dir, nil
}

// bootstrap puts the materialized directory on sys.path and runs a module of
// the package as "python -m" would. It is a -c program rather than PYTHONPATH
// because the interpreter runs with -E, which ignores PYTHON* variables so
// that the user's environment cannot change what the checker imports.
const bootstrap = `import sys, runpy; sys.path.insert(0, sys.argv.pop(1)); runpy.run_module(sys.argv.pop(1), run_name="__main__", alter_sys=True)`

// The modules the launcher runs: the checker, and what a REPL asks at a value
// prompt.
const (
	ModuleCheck = "csdfbmc"
	ModuleNext  = "csdfbmc.next"
)

// PythonArgs returns the arguments that run the checker in dir with args, to
// be passed to a python3 executable: no environment variables (-E), no user
// site (-s), no bytecode written next to the sources (-B).
func PythonArgs(dir string, args ...string) []string {
	return ModuleArgs(dir, ModuleCheck, args...)
}

// ModuleArgs is PythonArgs for the given module of the package.
func ModuleArgs(dir, module string, args ...string) []string {
	return append([]string{"-E", "-s", "-B", "-c", bootstrap, dir, module}, args...)
}
