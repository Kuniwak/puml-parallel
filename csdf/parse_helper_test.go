package csdf

import "fmt"

// parseAccepting reads content as a plain diagram: its syntax tree, then
// the plain diagram the tree says. Its clauses are left to the clause
// language's own tests.
func parseAccepting(content string) (*Diagram, error) {
	tree, err := Parse("", content)
	if err != nil {
		return nil, err
	}
	return Plain(tree)
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

// plainParser reads a plain diagram as parseAccepting does.
type plainParser struct{ input string }

func (p plainParser) parse() (*Diagram, error) { return parseAccepting(p.input) }

// newParser reads input as parseAccepting does.
func newParser(input string) plainParser {
	return plainParser{input: input}
}
