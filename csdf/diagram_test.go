package csdf

import (
	"errors"
	"testing"
)

// The parse error says the fact; naming the tool that expands a promotion is the
// tool layer's job (tools.UserFacingError).
func TestParseReportsThatTheSourceStillHoldsDirectives(t *testing.T) {
	cases := map[string]string{
		"a <<promote>> block": `@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
		"a sync note": `@startuml A
state "稼働中" as running
running : accounts
[*] --> running
note as n1
  sync BOOK : accounts(口座ID)
end note
@enduml
`,
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(source)
			if err == nil {
				t.Fatal("Parse() error = nil, want a parse error")
			}
			var hinted *PromotionHintError
			if !errors.As(err, &hinted) {
				t.Errorf("Parse() error = %q, want a *PromotionHintError in the chain", err)
			}
		})
	}
}

func TestParseDoesNotReportDirectivesForAnOrdinaryMistake(t *testing.T) {
	cases := map[string]string{
		"a malformed edge": `@startuml A
state "稼働中" as running
[*] --> running
running --> ; NO-EVENT
@enduml
`,
		// A local diagram wraps its PlantUML-only lines in CSDF-IGNORE, which
		// is exactly where an !include of a skin or a theme goes. What is
		// ignored cannot be a directive.
		"an !include inside a CSDF-IGNORE region": `@startuml A
' CSDF-IGNORE-BEGIN
!include style/theme.puml
' CSDF-IGNORE-END
state "稼働中" as running
[*] --> running
running --> ; NO-EVENT
@enduml
`,
		// A note is PlantUML's, not promotion's. The advice for one that was
		// not wrapped is CSDF-IGNORE, not csdfpromote.
		"a plain note": `@startuml A
state "稼働中" as running
[*] --> running
note left of running
  この図は promotion とは関係がない
end note
@enduml
`,
	}

	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(source)
			if err == nil {
				t.Fatal("Parse() error = nil, want a parse error")
			}
			var hinted *PromotionHintError
			if errors.As(err, &hinted) {
				t.Errorf("Parse() error = %q, want no *PromotionHintError in the chain", err)
			}
		})
	}
}

// The hint is added to the error, not put in place of it, so a caller can still
// tell what the parser actually said.
func TestParseKeepsTheParseErrorUnderTheHint(t *testing.T) {
	_, err := Parse(`@startuml A
state "稼働中" as running {
  running : accounts
  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`)
	if err == nil {
		t.Fatal("Parse() error = nil, want a parse error")
	}
	var hinted *PromotionHintError
	if !errors.As(err, &hinted) {
		t.Fatalf("Parse() error = %v, want a *PromotionHintError in the chain", err)
	}
	if errors.Unwrap(hinted) == nil {
		t.Error("PromotionHintError wraps nothing, want the parse error under it")
	}
}

func TestBlankIgnoreRegionsKeepsLineNumbers(t *testing.T) {
	// Setup
	source := "a\n' CSDF-IGNORE-BEGIN\nb\n' CSDF-IGNORE-END\nc"
	want := "a\n\n\n\nc"

	// Execute
	got := BlankIgnoreRegions(source)

	// Assert
	if got != want {
		t.Errorf("BlankIgnoreRegions = %q, want %q", got, want)
	}
}

func TestCheckIgnoreRegions(t *testing.T) {
	cases := []struct {
		name, source string
		wantErr      bool
	}{
		{"no region", "a\nb", false},
		{"a closed region", "' CSDF-IGNORE-BEGIN\nx\n' CSDF-IGNORE-END\n", false},
		{"an unterminated region", "a\n' CSDF-IGNORE-BEGIN\nx\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			err := CheckIgnoreRegions(c.source)

			// Assert
			if (err != nil) != c.wantErr {
				t.Errorf("CheckIgnoreRegions: err = %v, want an error: %v", err, c.wantErr)
			}
		})
	}
}
