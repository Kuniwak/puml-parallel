// Package engineload gives the exploration engine a page of replweb
// carries: from a file, embedded in the binary, or built by the Go
// toolchain from the module's source.
package engineload

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/v8/tools/replweb"
)

// Source gives an engine.
type Source func() (replweb.Engine, error)

// File is the Source that reads the engine from path, a WebAssembly file
// either gzipped or not.
func File(path string) Source {
	return func() (replweb.Engine, error) {
		bs, err := os.ReadFile(path)
		if err != nil {
			return replweb.Engine{}, fmt.Errorf("engineload.File: %w", err)
		}
		gz, err := asGzippedWasm(bs)
		if err != nil {
			return replweb.Engine{}, fmt.Errorf("engineload.File: %w", fmt.Errorf("%s: %w", path, err))
		}
		return replweb.Engine{WasmGz: gz}, nil
	}
}

func asGzippedWasm(bs []byte) ([]byte, error) {
	switch {
	case bytes.HasPrefix(bs, []byte{0x1f, 0x8b}):
		return bs, nil
	case bytes.HasPrefix(bs, []byte("\x00asm")):
		return Gzip(bs)
	default:
		return nil, errors.New("not a WebAssembly file, gzipped or not")
	}
}

// Gzip compresses bs with no name or time in the header, so the same input
// gives the same bytes.
func Gzip(bs []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(bs); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Choose is the Source a page takes its engine from: the file at path if
// one is given, else the engine embedded in the binary if there is one
// (embedded is not nil), else build.
func Choose(path string, embedded, build Source) Source {
	switch {
	case path != "":
		return File(path)
	case embedded != nil:
		return embedded
	default:
		return build
	}
}
