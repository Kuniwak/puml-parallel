package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
)

func TestSelfTransitionDivergenceRuleReportsAnEdgeBackToItsOwnState(t *testing.T) {
	// Arrange: one self-transition among ordinary ones.
	source := strings.Join([]string{
		"@startuml",
		`state "s0" as s0`,
		`state "s1" as s1`,
		"[*] --> s0",
		"s0 --> s0 : a",
		"s0 --> s1 : b",
		"@enduml",
		"",
	}, "\n")

	// Act
	findings := lint.Run([]lint.Rule{lint.SelfTransitionDivergenceRule{}}, lint.NewInput("a.puml", []byte(source)))

	// Assert
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
	}
	got := findings[0]
	if got.Severity != lint.SeverityHint {
		t.Errorf("want HINT, got %s", got.Severity)
	}
	if got.RuleID != "self-transition-divergence" {
		t.Errorf("want self-transition-divergence, got %s", got.RuleID)
	}
	if got.StartLine != 5 || got.EndLine != 5 {
		t.Errorf("want lines 5-5, got %d-%d", got.StartLine, got.EndLine)
	}
	if !strings.Contains(got.Message, "`a`") || !strings.Contains(got.Message, "`s0`") {
		t.Errorf("want the event and the state named, got %q", got.Message)
	}
}

func TestSelfTransitionPostBreaksGuardRuleReportsASelfTransitionWithBothAGuardAndAPost(t *testing.T) {
	// Arrange: one self-transition that has both, one that has neither.
	source := strings.Join([]string{
		"@startuml",
		`state "s0" as s0`,
		"s0 : n ; Nat",
		"[*] --> s0",
		"s0 --> s0 : a ; n = 0 ; n' = n + 1",
		"s0 --> s0 : b",
		"@enduml",
		"",
	}, "\n")

	// Act
	findings := lint.Run([]lint.Rule{lint.SelfTransitionPostBreaksGuardRule{}}, lint.NewInput("a.puml", []byte(source)))

	// Assert
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
	}
	got := findings[0]
	if got.Severity != lint.SeverityHint {
		t.Errorf("want HINT, got %s", got.Severity)
	}
	if got.RuleID != "self-transition-post-breaks-guard" {
		t.Errorf("want self-transition-post-breaks-guard, got %s", got.RuleID)
	}
	if got.StartLine != 5 || got.EndLine != 5 {
		t.Errorf("want lines 5-5, got %d-%d", got.StartLine, got.EndLine)
	}
	if !strings.Contains(got.Message, "n = 0") || !strings.Contains(got.Message, "n' = n + 1") {
		t.Errorf("want the guard and the post quoted, got %q", got.Message)
	}
}
