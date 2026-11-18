package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/lint"
	"github.com/google/go-cmp/cmp"
)

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
	// A glue refusing every atom whose text holds LIMIT.
	asked := 0
	refusing := func(atom clause.AtomWire, role lint.Role) (string, error) {
		asked++
		if strings.Contains(strings.Join(atom.Text, ""), "LIMIT") {
			return "not a " + string(role), nil
		}
		return "", nil
	}

	// Execute
	findings := lint.Run([]lint.Rule{lint.AtomNotInLanguageRule{Lint: refusing}}, lint.NewInput("a.puml", []byte(source)))

	// Assert
	message := "The guard atom `$n > LIMIT` is not one the glue reads: not a guard. Write it in the glue's language, or fix the glue's arguments if it reads a name they should define."
	want := []lint.Finding{
		{File: "a.puml", StartLine: 5, EndLine: 5, RuleID: "atom-not-in-language", Severity: lint.SeverityError, Message: message},
		{File: "a.puml", StartLine: 6, EndLine: 6, RuleID: "atom-not-in-language", Severity: lint.SeverityError, Message: message},
	}
	if diff := cmp.Diff(want, findings); diff != "" {
		t.Errorf("findings mismatch (-want +got):\n%s", diff)
	}
	if asked != 3 {
		t.Errorf("want each distinct atom asked once, 3 in all, got %d", asked)
	}
}
