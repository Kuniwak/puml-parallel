package csdftracechkcmd

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
)

// MatchName is the -match spelling of a tracechk.Match. The spelling is a
// concern of the command line, so the table lives here and the core stays open
// to rules it does not name.
type MatchName string

const (
	MatchNameExact  MatchName = "exact"
	MatchNamePrefix MatchName = "prefix"
)

var matchRules = map[MatchName]tracechk.Match{
	MatchNameExact:  tracechk.MatchExact,
	MatchNamePrefix: tracechk.MatchPrefix,
}

// ParseMatchName reads a -match spelling.
func ParseMatchName(s string) (MatchName, error) {
	if _, ok := matchRules[MatchName(s)]; !ok {
		return "", fmt.Errorf("unknown match rule %q (want exact or prefix)", s)
	}
	return MatchName(s), nil
}

// Rule is the tracechk.Match the name stands for.
func (n MatchName) Rule() tracechk.Match { return matchRules[n] }
