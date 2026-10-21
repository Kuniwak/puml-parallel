package clause

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// HoleMark is how the name of an environment function writes a hole.
const HoleMark = "$_"

// FunctionOf is the uninterpreted function an atom denotes
// (docs/CLAUSE_DESIGN.md, P15): its name is the atom's text without its
// holes, each written HoleMark, with a space after one where the text goes
// on with a character that could continue a name, and a $ of the text that
// would start a hole escaped; its arguments are the atom's arguments, the
// distinct terms its holes read in order of first appearance (P9). Atoms
// with the same text denote the same function, whatever their holes read.
// csdfbmc names the function alike (csdfbmc/atoms.py).
func FunctionOf(a Atom) (string, []Term, error) {
	w, terms, err := Wire(a)
	if err != nil {
		return "", nil, fmt.Errorf("clause.FunctionOf: %w", err)
	}
	var sb strings.Builder
	sb.WriteString(escapeHoleStarts(w.Text[0]))
	for _, piece := range w.Text[1:] {
		sb.WriteString(HoleMark)
		if piece != "" && ident.IsRune(firstRune(piece)) {
			sb.WriteString(" ")
		}
		sb.WriteString(escapeHoleStarts(piece))
	}
	return sb.String(), terms, nil
}

// escapeHoleStarts escapes every $ of piece that would start a hole.
func escapeHoleStarts(piece string) string {
	rs := []rune(piece)
	var sb strings.Builder
	for i, r := range rs {
		if r == '$' && i+1 < len(rs) && (rs[i+1] == '{' || startsHoleName(rs[i+1])) {
			sb.WriteString(`\$`)
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
