// Package json compiles the livelock-freedom obligation IR to its JSON encoding
// (the "ir-json" target), the canonical wire form also emitted by csdflivelockfree.
package irjson

import (
	stdjson "encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

func CompileLivelockFree(w io.Writer, r io.Reader) error {
	input, err := io.ReadAll(r)
	d, err := csdf.ParseBytes(input)
	if err != nil {
		return fmt.Errorf("isabelle.Compile: %w", err)
	}
	WriteLivelockFree(w, obligationir.BuildLivelockFree(d))
	return nil
}

func MustCompileLivelockFree(w io.Writer, r io.Reader) {
	if err := CompileLivelockFree(w, r); err != nil {
		panic(fmt.Sprintf("irjson.MustCompileLivelockFree: %v", err))
	}
}

func CompileLivelockFreeString(input string) (string, error) {
	d, err := csdf.Parse(input)
	if err != nil {
		return "", fmt.Errorf("isabelle.Compile: %w", err)
	}
	var b strings.Builder
	WriteLivelockFree(&b, obligationir.BuildLivelockFree(d))
	return b.String(), nil
}

func MustCompileLivelockFreeString(input string) string {
	s, err := CompileLivelockFreeString(input)
	if err != nil {
		panic(fmt.Sprintf("irjson.MustCompileLivelockFreeString: %v", err))
	}
	return s
}

// WriteLivelockFree writes ir to w as newline-terminated JSON.
func WriteLivelockFree(w io.Writer, ir obligationir.IRLivelockFree) error {
	return stdjson.NewEncoder(w).Encode(ir)
}
