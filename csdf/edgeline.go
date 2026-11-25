package csdf

import "fmt"

// ParseEdgeLine reads one edge, "src --> dst : label", written at line of a
// source, its predicates left as written: for a reader of a syntax built on
// this one, such as a global diagram's hand-written edges, so that it reads
// an edge label as every diagram does.
func ParseEdgeLine(text string, line int) (Edge, error) {
	p := NewParser(text + "\n")
	p.line = line
	e, err := p.parseEdge()
	if err != nil {
		return Edge{}, fmt.Errorf("csdf.ParseEdgeLine: %w", err)
	}
	return edgeOf(e), nil
}
