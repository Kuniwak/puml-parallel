// Package lint checks a Composable State Diagram and reports what is wrong with
// it, or what only looks wrong. It has two readers: a person or a CI job, which
// wants a verdict, and an AI asked to review its own diagram, which wants to be
// told what to look at. Both read the same findings, so a message says what is
// suspicious and what would settle it, not just that something is.
//
// The rules are a slice, so adding one is adding a Rule to DefaultRules.
package lint

// Severity says how sure a finding is, and so what a reader owes it.
type Severity string

const (
	// SeverityError is a finding that is definitely wrong.
	SeverityError Severity = "ERROR"
	// SeverityWarn is a finding that is probably not what the author meant.
	SeverityWarn Severity = "WARN"
	// SeverityHint is a finding that is suspicious but may well be intended.
	SeverityHint Severity = "HINT"
	// SeverityStyleProblem is a finding about how the source is written rather
	// than about what it means.
	SeverityStyleProblem Severity = "STYLE_PROBLEM"
)

// Finding is one thing a rule has to say about one span of one file.
type Finding struct {
	File      string
	StartLine int
	EndLine   int
	RuleID    string
	Severity  Severity
	Message   string
}
