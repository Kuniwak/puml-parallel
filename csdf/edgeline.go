package csdf

import "fmt"

// ParseEdgeLine reads one edge, "src --> dst : label", written at line of a
// source, into its syntax tree: for a reader of a syntax built on
// this one, such as a global diagram's hand-written edges, so that it reads
// an edge label as every diagram does.
func ParseEdgeLine(text string, line int) (*EdgeSyntax, error) {
	p := newSourceParser(text + "\n")
	p.line = line
	e, err := p.parseEdge()
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseEdgeLine: %w", err)
	}
	return e, nil
}
