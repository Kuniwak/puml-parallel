package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

// A constrain's guard is written in the language a glue reads, so a formal:
// line is refused with what to write instead, wherever it stands.
func TestParseGlobalRefusesAFormalLine(t *testing.T) {
	cases := map[string]string{
		"under a constrain": "  constrain DEPOSIT?口座ID?額 ; 額 は最小単位の倍数\n  formal: 額 % UNIT == 0\n",
		"under a sync":      "  sync DEPOSIT : accounts(口座ID), accounts(口座ID)\n  formal: True\n",
	}
	for name, note := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			source := `@startuml ACCOUNTS
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空

note as c1
` + note + `end note
@enduml
`

			// Execute
			_, err := promote.ParseGlobal(source)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "glue") {
				t.Errorf("promote.ParseGlobal() error = %v, want the formal line refused, pointing at a glue", err)
			}
		})
	}
}
