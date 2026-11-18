package engineload

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/tools/replweb"
)

// enginePackage is the package of the engine, relative to the module root.
const enginePackage = "./tools/replweb/engine"

// ErrNoGo is the error of a Go that cannot run because no go command is on
// the PATH.
var ErrNoGo = errors.New("no go command is on the PATH to build the engine with")

// Go runs the go command.
type Go interface {
	// Run runs go with args in dir, with env added to the environment, and
	// returns what it writes on its standard output.
	Run(dir string, env []string, args ...string) ([]byte, error)
}

// ExecGo is the Go that runs the go command on the PATH.
type ExecGo struct{}

func (ExecGo) Run(dir string, env []string, args ...string) ([]byte, error) {
	bin, err := exec.LookPath("go")
	if err != nil {
		return nil, ErrNoGo
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Module is the main module of a binary: its path and version.
type Module struct {
	Path, Version string
}

// ThisModule is the main module of the running binary.
func ThisModule() Module {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Module{}
	}
	return Module{Path: info.Main.Path, Version: info.Main.Version}
}

// released tells whether v names a version the module proxy may hold: not
// a build from a work tree.
func released(v string) bool {
	return v != "" && v != "(devel)" && !strings.HasSuffix(v, "+dirty")
}

// Builder builds the engine with Go from the source of Module: of the
// version it names, from the module cache, if it is a released one, else
// from the work tree of the module WorkDir is in. It keeps what it built
// from a released version under CacheDir/<version>, unless CacheDir is "".
type Builder struct {
	Go       Go
	Module   Module
	WorkDir  string
	CacheDir string
}

// Source is the Source that builds the engine.
func (b Builder) Source() Source {
	return func() (replweb.Engine, error) {
		engine, err := b.engine()
		if err != nil {
			return replweb.Engine{}, fmt.Errorf("engineload.Builder: %w", err)
		}
		return engine, nil
	}
}

func (b Builder) engine() (replweb.Engine, error) {
	if released(b.Module.Version) {
		if engine, ok := b.cached(); ok {
			return engine, nil
		}
		if dir, err := b.download(); err == nil {
			engine, err := b.build(dir, b.Module.Version)
			if err != nil {
				return replweb.Engine{}, err
			}
			b.keep(engine)
			return engine, nil
		} else if errors.Is(err, ErrNoGo) {
			return replweb.Engine{}, err
		}
	}
	dir, err := b.workTree()
	if err != nil {
		return replweb.Engine{}, err
	}
	return b.build(dir, "")
}

func (b Builder) download() (string, error) {
	out, err := b.Go.Run(b.WorkDir, []string{"GOWORK=off"}, "mod", "download", "-json", b.Module.Path+"@"+b.Module.Version)
	if err != nil {
		return "", err
	}
	var mod struct{ Dir string }
	if err := json.Unmarshal(out, &mod); err != nil {
		return "", err
	}
	if mod.Dir == "" {
		return "", fmt.Errorf("go mod download gave no directory for %s@%s", b.Module.Path, b.Module.Version)
	}
	return mod.Dir, nil
}

func (b Builder) workTree() (string, error) {
	out, err := b.Go.Run(b.WorkDir, []string{"GOWORK=off"}, "list", "-m", "-f", "{{.Path}} {{.Dir}}")
	if errors.Is(err, ErrNoGo) {
		return "", err
	}
	path, dir, ok := strings.Cut(strings.TrimSpace(string(out)), " ")
	if err != nil || !ok || path != b.Module.Path {
		return "", fmt.Errorf("this binary was built from a work tree of %s (version %q): run it inside that work tree to build the engine from it, or give the engine with -engine", b.Module.Path, b.Module.Version)
	}
	return dir, nil
}

func (b Builder) build(dir, version string) (replweb.Engine, error) {
	return Build(b.Go, dir, b.Module.Path, version)
}

// Build builds the engine with g in dir, the root of the module modulePath,
// stamping it with version unless version is "".
func Build(g Go, dir, modulePath, version string) (replweb.Engine, error) {
	tmp, err := os.MkdirTemp("", "csdfreplweb-engine")
	if err != nil {
		return replweb.Engine{}, err
	}
	defer os.RemoveAll(tmp)
	wasm := filepath.Join(tmp, "engine.wasm")
	ldflags := "-s -w"
	if version != "" {
		ldflags += " -X " + modulePath + "/version.Version=" + version
	}
	env := []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0", "GOWORK=off"}
	if _, err := g.Run(dir, env, "build", "-trimpath", "-buildvcs=false", "-ldflags="+ldflags, "-o", wasm, enginePackage); err != nil {
		return replweb.Engine{}, err
	}
	raw, err := os.ReadFile(wasm)
	if err != nil {
		return replweb.Engine{}, err
	}
	gz, err := Gzip(raw)
	if err != nil {
		return replweb.Engine{}, err
	}
	goroot, err := g.Run(dir, env, "env", "GOROOT")
	if err != nil {
		return replweb.Engine{}, err
	}
	glue, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return replweb.Engine{}, err
	}
	return replweb.Engine{WasmGz: gz, Glue: glue}, nil
}

func (b Builder) cacheDir() string {
	return filepath.Join(b.CacheDir, b.Module.Version)
}

func (b Builder) cached() (replweb.Engine, bool) {
	if b.CacheDir == "" {
		return replweb.Engine{}, false
	}
	gz, err := os.ReadFile(filepath.Join(b.cacheDir(), "engine.wasm.gz"))
	if err != nil {
		return replweb.Engine{}, false
	}
	glue, err := os.ReadFile(filepath.Join(b.cacheDir(), "wasm_exec.js"))
	if err != nil {
		return replweb.Engine{}, false
	}
	return replweb.Engine{WasmGz: gz, Glue: glue}, true
}

// keep caches engine; a cache that cannot be written is only slower.
func (b Builder) keep(engine replweb.Engine) {
	if b.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(b.cacheDir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(b.cacheDir(), "wasm_exec.js"), engine.Glue, 0o644)
	_ = os.WriteFile(filepath.Join(b.cacheDir(), "engine.wasm.gz"), engine.WasmGz, 0o644)
}
