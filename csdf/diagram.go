package csdf

import (
	"errors"
	"fmt"
	"strings"
)

// PromotionHintError is a refusal of a diagram that still holds promotion
// directives: its edges are not the whole of its behaviour, so reading it as
// a plain diagram would be worse than failing (Plain). The hint says what the
// author is missing rather than leaving them to read a column number. It wraps
// the refusal, so a caller can still reach it.
//
// It says the fact and not what to run: which tool expands a promotion is the
// tool layer's business, and this package is the grammar.
type PromotionHintError struct{ err error }

func (e *PromotionHintError) Error() string {
	return e.err.Error() + ": the source holds promotion directives, so its edges are not the whole of its behaviour"
}

func (e *PromotionHintError) Unwrap() error { return e.err }

// UserFacing marks this error's own message as the one to show, so that the
// hint is not unwrapped away on its road to the terminal.
func (e *PromotionHintError) UserFacing() {}

// ParseError is a parse failure at a known position in the source. The parser
// already writes the position into its message; this type carries it apart from
// the prose as well, so that a caller that has to point at the source - a lint
// reporting a file and a line - does not have to read it back out of English.
//
// Its own message is the message of the error it wraps, so wrapping changes
// nothing a reader sees.
type ParseError struct {
	Line int // 1-based line the parser stopped at.
	Col  int // 1-based column the parser stopped at.
	err  error
}

func (e *ParseError) Error() string { return e.err.Error() }

func (e *ParseError) Unwrap() error { return e.err }

// NewParseError is err tagged with the position line and col it is at; col is
// 0 when the position is a whole line, such as that of a clause, whose own
// position is not the line's.
func NewParseError(line, col int, err error) *ParseError {
	return &ParseError{Line: line, Col: col, err: err}
}

// Parse reads the concrete syntax tree of the diagram in source, plain or
// global, nothing of its meaning read; path is the file source was read
// from, "" when it is not known. A failure is a *ParseError tagged
// with the position the parser stopped at, unless it already says where it
// is.
func Parse(path, source string) (*SyntaxTree, error) {
	p := newSourceParser(source)
	tree, err := p.parse()
	if err != nil {
		var tagged *ParseError
		if errors.As(err, &tagged) {
			return nil, err
		}
		return nil, &ParseError{Line: p.line, Col: p.col, err: err}
	}
	tree.Path = path
	return tree, nil
}

// BlankIgnoreRegions is source with every CSDF-IGNORE region, markers
// included, replaced by empty lines, so that what is left keeps its line
// numbers. An unterminated region runs to the end of the source, which the
// parser refuses anyway.
func BlankIgnoreRegions(source string) string {
	var kept []string
	ignoring := false
	for line := range strings.SplitSeq(source, "\n") {
		switch strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "'")) {
		case ignoreBeginMarker:
			ignoring = true
			kept = append(kept, "")
			continue
		case ignoreEndMarker:
			ignoring = false
			kept = append(kept, "")
			continue
		}
		if ignoring {
			line = ""
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// CheckIgnoreRegions reports a CSDF-IGNORE region that is never closed, an
// error by docs/SYNTAX.md, for readers that blank the regions themselves
// with BlankIgnoreRegions instead of parsing.
func CheckIgnoreRegions(source string) error {
	open := 0
	for i, line := range strings.Split(source, "\n") {
		switch strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "'")) {
		case ignoreBeginMarker:
			if open == 0 {
				open = i + 1
			}
		case ignoreEndMarker:
			open = 0
		}
	}
	if open > 0 {
		return fmt.Errorf("csdf.CheckIgnoreRegions: unterminated CSDF-IGNORE region at line %d", open)
	}
	return nil
}
