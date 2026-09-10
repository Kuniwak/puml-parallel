package lint

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
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
			userFacingMessage(in.ParseErr),
		),
	}}
}

// userFacing is implemented by an error whose own message is the one to show;
// unwrapping stops there. csdf.PromotionHintError is one.
type userFacing interface{ UserFacing() }

// userFacingMessage unwraps to the deepest error that is either the last one or
// a userFacing one, so that the internal package-qualified context of the
// wrapping does not reach a finding.
func userFacingMessage(err error) string {
	for {
		if _, ok := err.(userFacing); ok {
			return err.Error()
		}
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err.Error()
		}
		err = unwrapped
	}
}
