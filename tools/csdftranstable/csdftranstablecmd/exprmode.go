package csdftranstablecmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/transtable"
)

// ExprMode is the -expr-mode spelling of a transtable.Notation. The spelling is
// a concern of the command line, so the table lives here and the core stays
// open to notations it does not name.
type ExprMode string

const (
	ExprModeNatural ExprMode = "natural"
	ExprModeLogical ExprMode = "logical"
)

var notations = map[ExprMode]transtable.Notation{
	ExprModeNatural: transtable.NotationNatural,
	ExprModeLogical: transtable.NotationLogical,
}

// ParseExprMode reads an -expr-mode spelling.
func ParseExprMode(s string) (ExprMode, error) {
	if _, ok := notations[ExprMode(s)]; !ok {
		return "", fmt.Errorf("unknown expression mode %q (want natural or logical)", s)
	}
	return ExprMode(s), nil
}

// Notation is the transtable.Notation the mode stands for.
func (m ExprMode) Notation() transtable.Notation { return notations[m] }
