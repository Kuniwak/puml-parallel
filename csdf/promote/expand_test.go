package promote_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

// expand parses a global diagram and expands it against in-memory local
// diagrams, so that a test says what the local diagram is next to what it
// expands into.
func expand(t *testing.T, global string, locals map[string]string) (*promote.Expansion, []promote.Diagnostic) {
	t.Helper()

	g, err := promote.ParseGlobal(global)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}
	return promote.Expand(g, promote.MapLoader(locals), promote.DefaultTemplates())
}

func TestExpandOneMapWithNoStateVariables(t *testing.T) {
	got, diags := expand(t, `@startuml ACCOUNTS
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空
@enduml
`, map[string]string{
		"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
[*] --> accNone
accNone --> accOpen : OPEN
accOpen --> accNone : CLOSE
@enduml
`,
	})

	if len(diags) != 0 {
		t.Errorf("promote.Expand() diagnostics = %v, want none", diags)
	}

	want := `@startuml ACCOUNTS
state "稼働中" as running
running: accounts ; 口座ID ⇸ Account
[*] --> running : accounts は空
' promote: local/ACCOUNT.puml 〈開設済み〉 → 〈未開設〉
running --> running : CLOSE(口座ID) ; 口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉 ; accounts' = {口座ID} ⩤ accounts
' promote: local/ACCOUNT.puml 〈未開設〉 → 〈開設済み〉
running --> running : OPEN(口座ID) ; 口座ID ∉ dom accounts ; accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉}
@enduml
`
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("promote.Expand() mismatch (-want +got):\n%s", diff)
	}
}

// A map variable and the local state ids may be written in Japanese: the frame
// clauses must carry the names through unchanged.
func TestExpandMapVariableWrittenInJapanese(t *testing.T) {
	got, diags := expand(t, `@startuml ACCOUNTS
state "稼働中" as 稼働中 {
  稼働中 : 口座表 ; 口座ID ⇸ Account

  state "口座表 : 口座ID ⇸ Account" as 稼働中口座表 <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> 稼働中 : 口座表 は空
@enduml
`, map[string]string{
		"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as 未開設
state "開設済み" as 開設済み
[*] --> 未開設
未開設 --> 開設済み : OPEN
開設済み --> 未開設 : CLOSE
@enduml
`,
	})

	if len(diags) != 0 {
		t.Errorf("promote.Expand() diagnostics = %v, want none", diags)
	}

	want := `@startuml ACCOUNTS
state "稼働中" as 稼働中
稼働中: 口座表 ; 口座ID ⇸ Account
[*] --> 稼働中 : 口座表 は空
' promote: local/ACCOUNT.puml 〈開設済み〉 → 〈未開設〉
稼働中 --> 稼働中 : CLOSE(口座ID) ; 口座ID ∈ dom 口座表 ∧ 口座表(口座ID) ∈ 〈開設済み〉 ; 口座表' = {口座ID} ⩤ 口座表
' promote: local/ACCOUNT.puml 〈未開設〉 → 〈開設済み〉
稼働中 --> 稼働中 : OPEN(口座ID) ; 口座ID ∉ dom 口座表 ; 口座表' = 口座表 ∪ {口座ID ↦ 〈開設済み〉}
@enduml
`
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("promote.Expand() mismatch (-want +got):\n%s", diff)
	}
}
