package csdf

import "fmt"

// acceptClauses is the check of a test that reads the structure of a diagram
// and leaves its clauses to the clause language's own tests.
func acceptClauses(Predicate) error { return nil }

// parseAccepting reads content as ParseSource does, every clause accepted.
func parseAccepting(content string) (*Diagram, error) {
	return ParseSource(content, acceptClauses)
}

// mustParseAccepting is parseAccepting for a source a test knows is well
// formed.
func mustParseAccepting(content string) *Diagram {
	d, err := parseAccepting(content)
	if err != nil {
		panic(fmt.Errorf("csdf.mustParseAccepting: %w", err))
	}
	return d
}

// newParser reads input with every clause accepted.
func newParser(input string) *Parser {
	return NewParserCheckingClauses(input, acceptClauses)
}
