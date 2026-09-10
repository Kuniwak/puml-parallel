package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
)

// TestMessagesNeedNoQuoting keeps every finding one line of the TSV. A message
// holding a tab, a newline or a double quote makes csv.Writer quote the field,
// and a quoted field may span lines: the table stops being readable with a
// glance or with cut(1), which is how both of its readers read it.
func TestMessagesNeedNoQuoting(t *testing.T) {
	// Arrange: a diagram that trips every rule that reads transitions, and one
	// that trips the rule that does not.
	sources := map[string]string{
		"a diagram that trips every rule reading transitions": strings.Join([]string{
			"@startuml",
			`state "働いている" as s0`,
			"[*] --> s0",
			`s0 --> s0 : a ; n = 0 ; n' = n + 1`,
			"s0 --> s0 : tau",
			"@enduml",
			"",
		}, "\n"),
		"a source that does not parse": "@startuml\nbogus\n@enduml\n",
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			// Act
			findings := lint.Run(lint.DefaultRules(), lint.NewInput("a.puml", []byte(source)))

			// Assert
			if len(findings) == 0 {
				t.Fatalf("want findings, got none")
			}
			for _, f := range findings {
				if strings.ContainsAny(f.Message, "\t\n\r\"") {
					t.Errorf("%s: the message needs quoting: %q", f.RuleID, f.Message)
				}
			}
		})
	}
}
