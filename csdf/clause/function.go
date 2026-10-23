package clause

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// HoleMark is how the name of an environment function writes a hole,
// followed by the position of the argument the hole reads.
const HoleMark = "$_"

// Function is the uninterpreted function an atom denotes: its name and the
// terms it is applied to.
type Function struct {
	Name string
	Args []Term
}

// FunctionOf is the uninterpreted function an atom denotes
// (docs/CLAUSE_DESIGN.md, P15): its name is the atom's text without its
// holes, each written HoleMark and the position of the argument it reads, with a space after one where the text goes
// on with a character that could continue a name, and a $ of the text that
// would start a hole escaped; its arguments are the atom's arguments, the
// distinct terms its holes read in order of first appearance (P9). Atoms
// with the same text denote the same function, whatever their holes read.
// csdfbmc reads the name from the diagram's JSON (core.AtomDoc).
func FunctionOf(a Atom) (Function, error) {
	w, terms, err := Wire(a)
	if err != nil {
		return Function{}, fmt.Errorf("clause.FunctionOf: %w", err)
	}
	var sb strings.Builder
	sb.WriteString(escapeHoleStarts(w.Text[0]))
	for i, piece := range w.Text[1:] {
		sb.WriteString(HoleMark)
		sb.WriteString(strconv.Itoa(w.Holes[i]))
		if piece != "" && ident.IsRune(firstRune(piece)) {
			sb.WriteString(" ")
		}
		sb.WriteString(escapeHoleStarts(piece))
	}
	if terms == nil {
		terms = []Term{}
	}
	return Function{Name: sb.String(), Args: terms}, nil
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
