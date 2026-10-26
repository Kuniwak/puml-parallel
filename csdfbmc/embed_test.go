package csdfbmc_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdfbmc"
)

func TestMaterializeWritesThePackageOnceUnderAContentHash(t *testing.T) {
	root := t.TempDir()

	first, err := csdfbmc.Materialize(root)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if !strings.HasPrefix(first, root) {
		t.Errorf("Materialize() = %q, want a directory under %q", first, root)
	}
	for _, name := range []string{"__init__.py", "__main__.py", "checks.py", "cli.py", "cone.py", "environment.py", "explore.py", "formal.py", "formula.py", "meaning.py", "model.py", "next.py", "protocol.py", "python_glue.py", "values.py"} {
		if _, err := os.Stat(filepath.Join(first, "csdfbmc", name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(first, "csdfbmc", "embed_test.go")); err == nil {
		t.Error("Materialize() wrote a Go file; only the Python sources belong there")
	}

	second, err := csdfbmc.Materialize(root)
	if err != nil {
		t.Fatalf("second Materialize() error = %v", err)
	}
	if second != first {
		t.Errorf("second Materialize() = %q, want the same directory %q", second, first)
	}
}

func TestMaterializedPackageRunsUnderPython(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not on the PATH")
	}
	dir, err := csdfbmc.Materialize(t.TempDir())
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	cmd := exec.Command(py, csdfbmc.PythonArgs(dir, "--help")...)
	cmd.Dir = t.TempDir() // not the repository: the materialized copy must stand alone
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "usage: csdfbmc") {
		t.Fatalf("python -m csdfbmc --help: %v\n%s", err, out)
	}
}
