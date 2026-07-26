package irjson

import (
	stdjson "encoding/json"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// WriteRefinement writes ir to w as newline-terminated JSON, under the same
// no-HTML-escaping rule as WriteLivelockFree.
func WriteRefinement(w io.Writer, ir obligationir.IRRefinement) error {
	enc := stdjson.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(ir)
}
