// Command buildengine builds the engine of the web page (tools/replweb/engine)
// for WebAssembly from the work tree it runs in, and writes it gzipped as
// engine.wasm.gz, with the wasm_exec.js of the Go toolchain that built it,
// into the directory -o names. A release runs it before building
// csdfreplweb with the embedengine tag, which embeds what it writes into
// tools/replweb/engineload/embedded.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kuniwak/puml-parallel/v8/tools/replweb/engineload"
)

func main() {
	out := flag.String("o", filepath.Join("tools", "replweb", "engineload", "embedded"), "directory to write engine.wasm.gz and wasm_exec.js into")
	version := flag.String("version", "", "version to stamp the engine with")
	flag.Parse()
	if err := run(*out, *version); err != nil {
		fmt.Fprintln(os.Stderr, "buildengine:", err)
		os.Exit(1)
	}
}

func run(out, version string) error {
	g := engineload.ExecGo{}
	mod, err := g.Run(".", []string{"GOWORK=off"}, "list", "-m", "-f", "{{.Path}} {{.Dir}}")
	if err != nil {
		return err
	}
	path, dir, _ := strings.Cut(strings.TrimSpace(string(mod)), " ")
	engine, err := engineload.Build(g, dir, path, version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "engine.wasm.gz"), engine.WasmGz, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "wasm_exec.js"), engine.Glue, 0o644)
}
