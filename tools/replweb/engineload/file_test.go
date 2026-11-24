package engineload

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const wasmMagic = "\x00asm\x01\x00\x00\x00"

func gunzip(t *testing.T, gz []byte) []byte {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	bs, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func TestFileGzipsAPlainWasm(t *testing.T) {
	// Setup
	path := filepath.Join(t.TempDir(), "engine.wasm")
	if err := os.WriteFile(path, []byte(wasmMagic), 0o644); err != nil {
		t.Fatal(err)
	}

	// Execute
	engine, err := File(path)()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got := gunzip(t, engine.WasmGz); string(got) != wasmMagic {
		t.Errorf("engine = %q, want the file gzipped", got)
	}
}

func TestFileTakesAGzippedWasmAsItIs(t *testing.T) {
	// Setup
	gz, err := Gzip([]byte(wasmMagic))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "engine.wasm.gz")
	if err := os.WriteFile(path, gz, 0o644); err != nil {
		t.Fatal(err)
	}

	// Execute
	engine, err := File(path)()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(engine.WasmGz, gz) {
		t.Error("the engine is not the file")
	}
}

func TestFileRefusesWhatIsNotWasm(t *testing.T) {
	// Setup
	path := filepath.Join(t.TempDir(), "engine.wasm")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err := File(path)()

	// Assert
	if err == nil {
		t.Error("want an error, got nil")
	}
}
