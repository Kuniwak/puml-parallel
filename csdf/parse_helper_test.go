package csdf

import "fmt"

// parseAccepting reads content as ParseSource does; its clauses are left
// to the clause language's own tests.
func parseAccepting(content string) (*Diagram, error) {
	return ParseSource(content)
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

// newParser reads input.
func newParser(input string) *Parser {
	return NewParser(input)
}
