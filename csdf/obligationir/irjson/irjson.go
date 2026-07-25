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
	if err != nil {
		return fmt.Errorf("irjson.CompileLivelockFree: %w", err)
	}
	d, err := csdf.ParseBytes(input)
	if err != nil {
		return fmt.Errorf("irjson.CompileLivelockFree: %w", err)
	}
	if err := WriteLivelockFree(w, obligationir.BuildLivelockFree(d)); err != nil {
		return fmt.Errorf("irjson.CompileLivelockFree: %w", err)
	}
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
		return "", fmt.Errorf("irjson.CompileLivelockFreeString: %w", err)
	}
	var b strings.Builder
	if err := WriteLivelockFree(&b, obligationir.BuildLivelockFree(d)); err != nil {
		return "", fmt.Errorf("irjson.CompileLivelockFreeString: %w", err)
	}
	return b.String(), nil
}

func MustCompileLivelockFreeString(input string) string {
	s, err := CompileLivelockFreeString(input)
	if err != nil {
		panic(fmt.Sprintf("irjson.MustCompileLivelockFreeString: %v", err))
	}
	return s
}

// WriteLivelockFree writes ir to w as newline-terminated JSON. HTML escaping is
// off: the predicate texts are natural language, where <, > and & are common
// (n > 0), and escaping them only obscures the text for the human or LLM that
// has to formalise it.
func WriteLivelockFree(w io.Writer, ir obligationir.IRLivelockFree) error {
	enc := stdjson.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(ir)
}
