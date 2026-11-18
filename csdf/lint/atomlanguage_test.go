package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/lint"
	"github.com/google/go-cmp/cmp"
)

// refusing is a glue that refuses every atom whose text holds bad, saying
// so with the role it was asked in, and counts what it was asked.
type refusing struct {
	bad   string
	asked int
}

func (r *refusing) Lint(atom clause.AtomWire, role string) (string, error) {
	r.asked++
	if strings.Contains(strings.Join(atom.Text, ""), r.bad) {
		return "not a " + role, nil
	}
	return "", nil
}

func TestAtomNotInLanguageRule(t *testing.T) {
	// Setup
	source := strings.Join([]string{
		"@startuml",
		`state "s0" as s0`,
		"s0 : n ; Nat",
		"[*] --> s0 : $n' == 0",
		"s0 --> s0 : a ; $n > LIMIT ; $n' == $n",
		"s0 --> s0 : b ; $n > LIMIT ; $n' == $n",
		"@enduml",
		"",
	}, "\n")
	glue := &refusing{bad: "LIMIT"}

	// Execute
	findings := lint.Run([]lint.Rule{lint.AtomNotInLanguageRule{Glue: glue}}, lint.NewInput("a.puml", []byte(source)))

	// Assert
	message := "The guard atom `$n > LIMIT` is not one the glue reads: not a guard. Write it in the glue's language, or fix the glue's arguments if it reads a name they should define."
	want := []lint.Finding{
		{File: "a.puml", StartLine: 5, EndLine: 5, RuleID: "atom-not-in-language", Severity: lint.SeverityError, Message: message},
		{File: "a.puml", StartLine: 6, EndLine: 6, RuleID: "atom-not-in-language", Severity: lint.SeverityError, Message: message},
	}
	if diff := cmp.Diff(want, findings); diff != "" {
		t.Errorf("findings mismatch (-want +got):\n%s", diff)
	}
	if glue.asked != 3 {
		t.Errorf("want each distinct atom asked once, 3 in all, got %d", glue.asked)
	}
}
