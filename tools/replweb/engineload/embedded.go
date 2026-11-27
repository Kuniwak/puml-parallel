//go:build embedengine

package engineload

import (
	_ "embed"

	"github.com/Kuniwak/puml-parallel/v9/tools/replweb"
)

// The files internal/buildengine writes; a release writes them before
// building with the embedengine tag.
var (
	//go:embed embedded/engine.wasm.gz
	embeddedWasmGz []byte
	//go:embed embedded/wasm_exec.js
	embeddedGlue []byte
)

// Embedded is the Source of the engine embedded in the binary.
func Embedded() Source {
	return func() (replweb.Engine, error) {
		return replweb.Engine{WasmGz: embeddedWasmGz, Glue: embeddedGlue}, nil
	}
}
