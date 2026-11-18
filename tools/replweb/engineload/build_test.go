package engineload

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const modulePath = "example.com/m/v3"

// fakeGo answers go mod download, go list -m and go env GOROOT as told,
// and writes a WebAssembly file where go build is told to, recording the
// directory each build runs in and the ldflags it gets.
type fakeGo struct {
	downloadDir string
	listOut     string
	goroot      string
	builds      []string
	ldflags     []string
}

func (g *fakeGo) Run(dir string, _ []string, args ...string) ([]byte, error) {
	switch args[0] {
	case "mod":
		if g.downloadDir == "" {
			return nil, errors.New("not found")
		}
		return []byte(`{"Dir":"` + g.downloadDir + `"}`), nil
	case "list":
		if g.listOut == "" {
			return nil, errors.New("not a module")
		}
		return []byte(g.listOut + "\n"), nil
	case "env":
		return []byte(g.goroot + "\n"), nil
	case "build":
		g.builds = append(g.builds, dir)
		g.ldflags = append(g.ldflags, args[3])
		return nil, os.WriteFile(args[5], []byte(wasmMagic), 0o644)
	}
	return nil, errors.New("unexpected go " + strings.Join(args, " "))
}

func newGoroot(t *testing.T) string {
	t.Helper()
	goroot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goroot, "lib", "wasm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"), []byte("glue"), 0o644); err != nil {
		t.Fatal(err)
	}
	return goroot
}

func TestBuilderBuildsAReleasedVersionFromTheModuleCache(t *testing.T) {
	// Setup
	g := &fakeGo{downloadDir: "/modcache/m@v3.1.0", goroot: newGoroot(t)}
	b := Builder{Go: g, Module: Module{Path: modulePath, Version: "v3.1.0"}, WorkDir: "/work"}

	// Execute
	engine, err := b.Source()()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"/modcache/m@v3.1.0"}, g.builds); diff != "" {
		t.Errorf("builds mismatch (-want +got):\n%s", diff)
	}
	if !strings.Contains(g.ldflags[0], "-X "+modulePath+"/version.Version=v3.1.0") {
		t.Errorf("ldflags = %q, want the version stamped", g.ldflags[0])
	}
	if string(gunzip(t, engine.WasmGz)) != wasmMagic || string(engine.Glue) != "glue" {
		t.Errorf("engine = %+v", engine)
	}
}

func TestBuilderBuildsADevelVersionFromTheWorkTree(t *testing.T) {
	for _, version := range []string{"(devel)", "", "v3.1.1-0.20261118000000-abcdefabcdef+dirty"} {
		t.Run(version, func(t *testing.T) {
			// Setup
			g := &fakeGo{listOut: modulePath + " /work/root", goroot: newGoroot(t)}
			b := Builder{Go: g, Module: Module{Path: modulePath, Version: version}, WorkDir: "/work/root/sub"}

			// Execute
			_, err := b.Source()()

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff([]string{"/work/root"}, g.builds); diff != "" {
				t.Errorf("builds mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuilderBuildsAVersionTheProxyDoesNotHoldFromTheWorkTree(t *testing.T) {
	// Setup
	g := &fakeGo{listOut: modulePath + " /work/root", goroot: newGoroot(t)}
	b := Builder{Go: g, Module: Module{Path: modulePath, Version: "v3.1.1-0.20261118000000-abcdefabcdef"}, WorkDir: "/work/root"}

	// Execute
	_, err := b.Source()()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"/work/root"}, g.builds); diff != "" {
		t.Errorf("builds mismatch (-want +got):\n%s", diff)
	}
}

func TestBuilderRefusesADevelVersionOutsideItsWorkTree(t *testing.T) {
	// Setup
	g := &fakeGo{listOut: "example.com/other /elsewhere", goroot: newGoroot(t)}
	b := Builder{Go: g, Module: Module{Path: modulePath, Version: "(devel)"}, WorkDir: "/elsewhere"}

	// Execute
	_, err := b.Source()()

	// Assert
	if err == nil || len(g.builds) != 0 {
		t.Errorf("error = %v after %d builds, want an error and no build", err, len(g.builds))
	}
}

func TestBuilderBuildsAReleasedVersionOnce(t *testing.T) {
	// Setup
	g := &fakeGo{downloadDir: "/modcache/m@v3.1.0", goroot: newGoroot(t)}
	b := Builder{Go: g, Module: Module{Path: modulePath, Version: "v3.1.0"}, CacheDir: t.TempDir()}

	// Execute
	first, err1 := b.Source()()
	second, err2 := b.Source()()

	// Assert
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if len(g.builds) != 1 {
		t.Errorf("built %d times, want once", len(g.builds))
	}
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("the cached engine differs (-first +second):\n%s", diff)
	}
}

func TestBuilderDoesNotCacheADevelVersion(t *testing.T) {
	// Setup
	g := &fakeGo{listOut: modulePath + " /work/root", goroot: newGoroot(t)}
	b := Builder{Go: g, Module: Module{Path: modulePath, Version: "(devel)"}, CacheDir: t.TempDir()}

	// Execute
	_, err1 := b.Source()()
	_, err2 := b.Source()()

	// Assert
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if len(g.builds) != 2 {
		t.Errorf("built %d times, want twice", len(g.builds))
	}
}

type noGo struct{}

func (noGo) Run(string, []string, ...string) ([]byte, error) { return nil, ErrNoGo }

func TestBuilderSaysWhenThereIsNoGo(t *testing.T) {
	for _, version := range []string{"v3.1.0", "(devel)"} {
		t.Run(version, func(t *testing.T) {
			// Setup
			b := Builder{Go: noGo{}, Module: Module{Path: modulePath, Version: version}}

			// Execute
			_, err := b.Source()()

			// Assert
			if !errors.Is(err, ErrNoGo) {
				t.Errorf("error = %v, want ErrNoGo", err)
			}
		})
	}
}

func TestBuilderBuildsTheEngineOfThisWorkTree(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the engine for WebAssembly")
	}
	// Setup
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	b := Builder{Go: ExecGo{}, Module: Module{Path: "github.com/Kuniwak/puml-parallel/v3", Version: "(devel)"}, WorkDir: wd}

	// Execute
	engine, err := b.Source()()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(gunzip(t, engine.WasmGz)), "\x00asm") || !strings.Contains(string(engine.Glue), "Go") {
		t.Error("the engine is not WebAssembly with its glue")
	}
}
