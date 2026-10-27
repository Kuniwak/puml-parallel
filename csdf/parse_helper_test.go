package csdf

import "fmt"

// acceptClauses is the check of a test that reads the structure of a diagram
// and leaves its clauses to the clause language's own tests.
func acceptClauses(Predicate) error { return nil }

// Parse reads content with every clause accepted, a failure hinted as
// coreio hints it.
func Parse(content string) (*Diagram, error) {
	d, err := NewParserCheckingClauses(content, acceptClauses).ParseAt()
	if err != nil {
		return nil, WithPromotionHint(err, content)
	}
	return d, nil
}

// MustParse is Parse for a source a test knows is well formed.
func MustParse(content string) *Diagram {
	d, err := Parse(content)
	if err != nil {
		panic(fmt.Errorf("csdf.MustParse: %w", err))
	}
	return d
}

// newParser reads input with every clause accepted.
func newParser(input string) *Parser {
	return NewParserCheckingClauses(input, acceptClauses)
}
