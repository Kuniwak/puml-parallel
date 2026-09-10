package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
)

func TestSyntaxErrorRule(t *testing.T) {
	type testCase struct {
		Source        string
		WantLine      int // 0 when no finding is wanted.
		WantInMessage string
	}

	testCases := map[string]testCase{
		"a line the grammar has no rule for is reported where the parser stopped (representative value)": {
			Source:        "@startuml\nstate \"s0\" as s0\nbogus\n@enduml\n",
			WantLine:      3,
			WantInMessage: "unexpected syntax",
		},
		"a source that parses gets no finding (lower boundary value)": {
			Source: "@startuml\nstate \"s0\" as s0\n[*] --> s0\n@enduml\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange & Act
			findings := lint.Run([]lint.Rule{lint.SyntaxErrorRule{}}, lint.NewInput("a.puml", []byte(testCase.Source)))

			// Assert
			if testCase.WantLine == 0 {
				if len(findings) != 0 {
					t.Fatalf("want no finding, got %v", findings)
				}
				return
			}
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
			if got.StartLine != testCase.WantLine || got.EndLine != testCase.WantLine {
				t.Errorf("want lines %d-%[1]d, got %d-%d", testCase.WantLine, got.StartLine, got.EndLine)
			}
			if !strings.Contains(got.Message, testCase.WantInMessage) {
				t.Errorf("want the parser's own message, got %q", got.Message)
			}
		})
	}
}
