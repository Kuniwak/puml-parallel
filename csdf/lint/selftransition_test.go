package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
)

func TestSelfTransitionRules(t *testing.T) {
	type testCase struct {
		Rule           lint.Rule
		Source         string
		WantRuleID     string // "" when no finding is wanted.
		WantLine       int
		WantInMessages []string
	}

	// Both rules read the same shape, so they are checked against the same
	// diagrams: what tells them apart is which edges each one speaks about.
	both := strings.Join([]string{
		"@startuml",
		`state "s0" as s0`,
		"s0 : n ; Nat",
		"[*] --> s0",
		"s0 --> s0 : a ; n = 0 ; n' = n + 1",
		"@enduml",
		"",
	}, "\n")
	bare := strings.Join([]string{
		"@startuml",
		`state "s0" as s0`,
		`state "s1" as s1`,
		"[*] --> s0",
		"s0 --> s0 : a",
		"s0 --> s1 : b",
		"@enduml",
		"",
	}, "\n")

	testCases := map[string]testCase{
		"divergence reports an edge back to its own state (representative value)": {
			Rule:           lint.SelfTransitionDivergenceRule{},
			Source:         bare,
			WantRuleID:     "self-transition-divergence",
			WantLine:       5,
			WantInMessages: []string{"`a`", "`s0`"},
		},
		"divergence says nothing about an edge to another state (lower boundary value)": {
			Rule:   lint.SelfTransitionDivergenceRule{},
			Source: "@startuml\nstate \"s0\" as s0\nstate \"s1\" as s1\n[*] --> s0\ns0 --> s1 : b\n@enduml\n",
		},
		"post-breaks-guard reports a self-transition with both a guard and a post (representative value)": {
			Rule:           lint.SelfTransitionPostBreaksGuardRule{},
			Source:         both,
			WantRuleID:     "self-transition-post-breaks-guard",
			WantLine:       5,
			WantInMessages: []string{"n = 0", "n' = n + 1"},
		},
		"post-breaks-guard says nothing about a self-transition with neither (lower boundary value)": {
			Rule:   lint.SelfTransitionPostBreaksGuardRule{},
			Source: bare,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange & Act
			findings := lint.Run([]lint.Rule{testCase.Rule}, lint.NewInput("a.puml", []byte(testCase.Source)))

			// Assert
			if testCase.WantRuleID == "" {
				if len(findings) != 0 {
					t.Fatalf("want no finding, got %v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
			}
			got := findings[0]
			if got.Severity != lint.SeverityHint {
				t.Errorf("want HINT, got %s", got.Severity)
			}
			if got.RuleID != testCase.WantRuleID {
				t.Errorf("want %s, got %s", testCase.WantRuleID, got.RuleID)
			}
			if got.StartLine != testCase.WantLine || got.EndLine != testCase.WantLine {
				t.Errorf("want lines %d-%[1]d, got %d-%d", testCase.WantLine, got.StartLine, got.EndLine)
			}
			for _, want := range testCase.WantInMessages {
				if !strings.Contains(got.Message, want) {
					t.Errorf("want %q named, got %q", want, got.Message)
				}
			}
		})
	}
}
