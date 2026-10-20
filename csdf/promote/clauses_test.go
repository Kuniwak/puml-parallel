package promote_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

// The clauses of a generated edge are recorded one by one, each saying what it
// is: a tool reading the expansion must not have to parse the wording back.
func TestExpandRecordsTheClausesOfEveryGeneratedEdge(t *testing.T) {
	got, diags := expand(t, `@startuml ACCOUNTS
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account
  running : day

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空

note as c1
  constrain DEPOSIT(口座ID, 額) ; 額 は最小単位の倍数
end note
@enduml
`, map[string]string{
		"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
accOpen: 残高
[*] --> accNone
accNone --> accOpen : OPEN ; true ; 残高' = 0
accOpen --> accOpen : DEPOSIT(額) ; 0 < 額 ∧ 額 ≤ 上限 ; 残高' = 残高 + 額
@enduml
`,
	})
	if len(diags) != 0 {
		t.Fatalf("promote.Expand() diagnostics = %v, want none", diags)
	}

	var deposit, open promote.ExpandedEdge
	for _, e := range got.Edges {
		switch e.Edge.Event {
		case "DEPOSIT(口座ID, 額)":
			deposit = e
		case "OPEN(口座ID)":
			open = e
		}
	}

	wantGuard := []promote.ExpandedClause{
		{Kind: promote.ClauseInDom, Text: "口座ID ∈ dom accounts", Map: "accounts", ID: "口座ID"},
		{Kind: promote.ClauseAtState, Text: "accounts(口座ID) ∈ 〈開設済み〉", Map: "accounts", ID: "口座ID", State: "開設済み"},
		{Kind: promote.ClauseLocal, Text: "0 < 額", Map: "accounts", Path: "local/ACCOUNT.puml", Line: 7},
		{Kind: promote.ClauseLocal, Text: "額 ≤ 上限", Map: "accounts", Path: "local/ACCOUNT.puml", Line: 7},
		{Kind: promote.ClauseConstrain, Text: "額 は最小単位の倍数", Line: 14},
	}
	if diff := cmp.Diff(wantGuard, deposit.Guard); diff != "" {
		t.Errorf("DEPOSIT guard clauses mismatch (-want +got):\n%s", diff)
	}
	wantPost := []promote.ExpandedClause{
		{Kind: promote.ClauseUpdate, Text: "accounts' = accounts ⊕ {口座ID ↦ 〈開設済み〉}", Map: "accounts", ID: "口座ID", State: "開設済み"},
		{Kind: promote.ClauseLocal, Text: "残高' = 残高 + 額", Map: "accounts", Path: "local/ACCOUNT.puml", Line: 7},
		{Kind: promote.ClauseFrame, Text: "day' = day", Var: "day"},
	}
	if diff := cmp.Diff(wantPost, deposit.Post); diff != "" {
		t.Errorf("DEPOSIT post clauses mismatch (-want +got):\n%s", diff)
	}
	if got, want := string(deposit.Edge.Guard), "口座ID ∈ dom accounts ∧ accounts(口座ID) ∈ 〈開設済み〉 ∧ 0 < 額 ∧ 額 ≤ 上限 ∧ 額 は最小単位の倍数"; got != want {
		t.Errorf("DEPOSIT guard text = %q, want the clauses joined by the connective %q", got, want)
	}

	wantOpenGuard := []promote.ExpandedClause{
		{Kind: promote.ClauseNotInDom, Text: "口座ID ∉ dom accounts", Map: "accounts", ID: "口座ID"},
	}
	if diff := cmp.Diff(wantOpenGuard, open.Guard); diff != "" {
		t.Errorf("OPEN guard clauses mismatch (-want +got):\n%s", diff)
	}
	if open.Post[0].Kind != promote.ClauseCreate || open.Post[0].State != "開設済み" {
		t.Errorf("OPEN post[0] = %+v, want a create clause into 開設済み", open.Post[0])
	}
}

// A hand-written edge has no generated clauses, so it records none.
func TestExpandRecordsNoClausesForAHandWrittenEdge(t *testing.T) {
	got, _ := expand(t, `@startuml MODES
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account
  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
state "停止中" as halted
halted : accounts ; 口座ID ⇸ Account
[*] --> running : accounts は空
running --> halted : HALT ; true ; accounts' = accounts
@enduml
`, map[string]string{
		"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
[*] --> accNone
accNone --> accOpen : OPEN
@enduml
`,
	})
	for _, e := range got.Edges {
		if e.Edge.Event == "HALT" && (len(e.Guard) != 0 || len(e.Post) != 0) {
			t.Errorf("HALT clauses = %v / %v, want none", e.Guard, e.Post)
		}
	}
}
