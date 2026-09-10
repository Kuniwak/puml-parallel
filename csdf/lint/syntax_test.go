package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
)

func TestSyntaxErrorRuleReportsWhereTheParserStopped(t *testing.T) {
	// Arrange: a line the CSDF grammar has no rule for, on the third line.
	source := "@startuml\nstate \"s0\" as s0\nbogus\n@enduml\n"

	// Act
	findings := lint.Run([]lint.Rule{lint.SyntaxErrorRule{}}, lint.NewInput("a.puml", []byte(source)))

	// Assert
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
	}
	got := findings[0]
	if got.Severity != lint.SeverityError {
		t.Errorf("want ERROR, got %s", got.Severity)
	}
	if got.RuleID != "syntax-error" {
		t.Errorf("want syntax-error, got %s", got.RuleID)
	}
	if got.File != "a.puml" {
		t.Errorf("want a.puml, got %s", got.File)
	}
	if got.StartLine != 3 || got.EndLine != 3 {
		t.Errorf("want lines 3-3, got %d-%d", got.StartLine, got.EndLine)
	}
	if !strings.Contains(got.Message, "unexpected syntax") {
		t.Errorf("want the parser's own message, got %q", got.Message)
	}
}

func TestSyntaxErrorRuleSaysNothingAboutASourceThatParses(t *testing.T) {
	// Arrange
	source := "@startuml\nstate \"s0\" as s0\n[*] --> s0\n@enduml\n"

	// Act
	findings := lint.Run([]lint.Rule{lint.SyntaxErrorRule{}}, lint.NewInput("a.puml", []byte(source)))

	// Assert
	if len(findings) != 0 {
		t.Errorf("want no finding, got %v", findings)
	}
}
