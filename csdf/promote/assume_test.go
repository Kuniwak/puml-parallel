package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

const assumeLocal = `@startuml ATTEMPT
state "未登録" as attNone
state "滞留" as attStuck
[*] --> attNone
attNone --> attStuck : SEND
attStuck --> attNone : RESEND
@enduml
`

// An assume note is the core's own syntax: the global diagram passes it through
// to the expansion, where the obligation compilers read it.
func TestExpandKeepsAnAssumeNote(t *testing.T) {
	got, diags := expand(t, `@startuml ATTEMPTS
state "稼働中" as running {
  running : attempts ; 試行ID ⇸ Attempt

  state "attempts : 試行ID ⇸ Attempt" as runningAttempts <<promote>> {
    !include local/ATTEMPT.puml
  }
}

[*] --> running : attempts は空

note as fair
  assume 再送を続ければいつかどれかの試行が確定する
end note
@enduml
`, map[string]string{"local/ATTEMPT.puml": assumeLocal})
	if len(diags) != 0 {
		t.Fatalf("promote.Expand() diagnostics = %v, want none", diags)
	}

	want := []csdf.Assumption{{ID: "fair", Text: "再送を続ければいつかどれかの試行が確定する", Line: 13}}
	if diff := cmp.Diff(want, got.Diagram.Assumptions); diff != "" {
		t.Errorf("expansion assumptions mismatch (-want +got):\n%s", diff)
	}
	if !strings.Contains(got.String(), "note as fair\n  assume 再送を続ければいつかどれかの試行が確定する\nend note\n") {
		t.Errorf("expansion does not print the assume note:\n%s", got.String())
	}
}

func TestParseGlobalRefusesAnAnchoredAssumeNote(t *testing.T) {
	_, err := promote.ParseGlobal(`@startuml ATTEMPTS
state "稼働中" as running {
  running : attempts ; 試行ID ⇸ Attempt
  state "attempts : 試行ID ⇸ Attempt" as runningAttempts <<promote>> {
    !include local/ATTEMPT.puml
  }
}
[*] --> running : attempts は空
note bottom of runningAttempts
  assume 再送を続ければいつかどれかの試行が確定する
end note
@enduml
`)
	if err == nil {
		t.Fatal("promote.ParseGlobal() error = nil, want a refusal of the anchored assume note")
	}
	if !strings.Contains(err.Error(), "line 9") || !strings.Contains(err.Error(), "assume") {
		t.Errorf("promote.ParseGlobal() error = %v, want it to name line 9 and assume", err)
	}
}

func TestParseGlobalRefusesAnAssumeNoteHoldingASecondDirective(t *testing.T) {
	_, err := promote.ParseGlobal(`@startuml ATTEMPTS
state "稼働中" as running {
  running : attempts ; 試行ID ⇸ Attempt
  state "attempts : 試行ID ⇸ Attempt" as runningAttempts <<promote>> {
    !include local/ATTEMPT.puml
  }
}
[*] --> running : attempts は空
note as fair
  assume 再送を続ければいつかどれかの試行が確定する
  constrain RESEND(試行ID) ; 真
end note
@enduml
`)
	if err == nil || !strings.Contains(err.Error(), "second directive") {
		t.Fatalf("promote.ParseGlobal() error = %v, want a refusal of the second directive", err)
	}
}

func TestParseGlobalWarnsAboutAnAssumeInTheWrongCase(t *testing.T) {
	g, err := promote.ParseGlobal(`@startuml ATTEMPTS
state "稼働中" as running {
  running : attempts ; 試行ID ⇸ Attempt
  state "attempts : 試行ID ⇸ Attempt" as runningAttempts <<promote>> {
    !include local/ATTEMPT.puml
  }
}
[*] --> running : attempts は空
note as fair
  Assume 再送を続ければいつかどれかの試行が確定する
end note
@enduml
`)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}
	if len(g.Diagnostics) != 1 || g.Diagnostics[0].Severity != promote.SeverityWarning || !strings.Contains(g.Diagnostics[0].Message, "assume") {
		t.Errorf("promote.ParseGlobal() diagnostics = %v, want one warning about the case of assume", g.Diagnostics)
	}
}
