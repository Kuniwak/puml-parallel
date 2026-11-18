// Command buildengine builds the engine of the web page (../../engine) for
// WebAssembly, gzips it into ../../assets/engine.wasm.gz and copies the
// wasm_exec.js of the Go toolchain that built it beside it. go generate
// runs it in tools/replweb. The output depends only on the sources and the
// Go version, so the test workflow checks the committed files are current.
package main

import (
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	tmp, err := os.MkdirTemp("", "buildengine")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	wasm := filepath.Join(tmp, "engine.wasm")

	build := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", wasm, "./engine")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("buildengine: go build failed: %w", err)
	}
	if err := gzipFile(wasm, filepath.Join("assets", "engine.wasm.gz")); err != nil {
		return err
	}

	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return fmt.Errorf("buildengine: go env GOROOT failed: %w", err)
	}
	glue, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return fmt.Errorf("buildengine: %w", err)
	}
	return os.WriteFile(filepath.Join("assets", "wasm_exec.js"), glue, 0o644)
}

// gzipFile writes src gzipped to dst, with no name or time in the header,
// so the same input gives the same bytes.
func gzipFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	w, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		out.Close()
		return err
	}
	if _, err := w.Write(raw); err != nil {
		out.Close()
		return err
	}
	if err := w.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
