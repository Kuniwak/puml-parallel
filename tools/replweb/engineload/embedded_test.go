//go:build embedengine

package engineload

import (
	"strings"
	"testing"
)

func TestEmbeddedIsTheEngineWithItsGlue(t *testing.T) {
	// Execute
	engine, err := Embedded()()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(gunzip(t, engine.WasmGz)), "\x00asm") || len(engine.Glue) == 0 {
		t.Error("the embedded engine is not WebAssembly with its glue")
	}
}
