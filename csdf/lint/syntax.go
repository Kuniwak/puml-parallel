package lint

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/usererr"
)

// SyntaxErrorRule reports a file the CSDF grammar cannot read. Nothing else can
// be checked about such a file, so this is the only finding it gets.
type SyntaxErrorRule struct{}

func (SyntaxErrorRule) ID() string { return "syntax-error" }

func (SyntaxErrorRule) Description() string {
	return "The file is not a Composable State Diagram the CSDF grammar can read."
}

func (r SyntaxErrorRule) Check(in *Input) []Finding {
	if in.ParseErr == nil {
		return nil
	}

	// A failure before the parser ran - an unreadable PNG - has no position of
	// its own, and line 0 says exactly that: the finding is about the file.
	line := 0
	var parseErr *csdf.ParseError
	if errors.As(in.ParseErr, &parseErr) {
		line = parseErr.Line
	}

	return []Finding{{
		File:      in.File,
		StartLine: line,
		EndLine:   line,
		RuleID:    r.ID(),
		Severity:  SeverityError,
		Message: fmt.Sprintf(
			"The parser stopped here: %s. Fix the syntax before reading any other finding: every other rule is skipped while the file does not parse. The grammar is in docs/SYNTAX.md.",
			usererr.Message(in.ParseErr),
		),
	}}
}
