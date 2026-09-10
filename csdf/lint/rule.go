package lint

import (
	"cmp"
	"slices"
)

// Rule is one check. Adding a check is writing a Rule and putting it in
// DefaultRules; nothing else in the package knows what the rules are.
type Rule interface {
	// ID is the stable identifier a finding of this rule carries. It is part of
	// the output, so it is what a reader greps for and what a suppression
	// mechanism would name.
	ID() string

	// Description says in one line what the rule looks for.
	Description() string

	// Check returns what the rule has to say about the input, in source order.
	Check(in *Input) []Finding
}

// DefaultRules returns every rule, in the order they were introduced.
func DefaultRules() []Rule {
	return []Rule{
		SyntaxErrorRule{},
		SelfTransitionDivergenceRule{},
		SelfTransitionPostBreaksGuardRule{},
	}
}

// Run applies the rules to the input and returns their findings in source
// order, ties broken by rule and message so that a run is reproducible.
func Run(rules []Rule, in *Input) []Finding {
	var findings []Finding
	for _, rule := range rules {
		findings = append(findings, rule.Check(in)...)
	}
	slices.SortStableFunc(findings, func(a, b Finding) int {
		if c := cmp.Compare(a.StartLine, b.StartLine); c != 0 {
			return c
		}
		if c := cmp.Compare(a.RuleID, b.RuleID); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})
	return findings
}

// RunAll applies the rules to every input and returns their findings, the
// inputs in the order they were given and each input's findings in source
// order. A front end that lints many files does not decide any of that, so it
// does not write this loop.
func RunAll(rules []Rule, inputs []*Input) []Finding {
	var findings []Finding
	for _, in := range inputs {
		findings = append(findings, Run(rules, in)...)
	}
	return findings
}

// HasError reports whether any finding is definitely wrong, which is what makes
// a run fail.
func HasError(findings []Finding) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool { return f.Severity == SeverityError })
}
