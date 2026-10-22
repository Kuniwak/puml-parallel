// Package json compiles the livelock-freedom obligation IR to its JSON encoding
// (the "ir-json" target), the canonical wire form also emitted by csdflivelockfree.
package irjson

import (
	stdjson "encoding/json"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// WriteLivelockFree writes ir to w as newline-terminated JSON. HTML escaping is
// off: the texts of atoms are natural language, where <, > and & are common
// (n > 0), and escaping them only obscures the text for the human or LLM that
// has to formalise it.
func WriteLivelockFree(w io.Writer, ir obligationir.LivelockFree) error {
	enc := stdjson.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(ir)
}
