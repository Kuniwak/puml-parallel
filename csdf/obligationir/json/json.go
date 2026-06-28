// Package json compiles the livelock-freedom obligation IR to its JSON encoding
// (the "ir-json" target), the canonical wire form also emitted by csdflivelockfree.
package json

import (
	stdjson "encoding/json"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// Compile writes ir to w as newline-terminated JSON.
func Compile(w io.Writer, ir obligationir.ObligationIR) error {
	return stdjson.NewEncoder(w).Encode(ir)
}
