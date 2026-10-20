package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/google/go-cmp/cmp"
)

const formalGlobal = `@startuml ACCOUNTS
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}

[*] --> running : accounts は空

note as c1
  constrain DEPOSIT(口座ID, 額) ; 額 は最小単位の倍数
  formal: 額 % UNIT == 0
end note
@enduml
`

const formalLocal = `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
accOpen: 残高
[*] --> accNone
accNone --> accOpen : OPEN ; true ; 残高' = 0
accOpen --> accOpen : DEPOSIT(額) ; 0 < 額 ; 残高' = 残高 + 額
@enduml
`

// A constrain may carry its formal twin on the next line of the note, and the
// expansion hands it on with the clause.
func TestConstrainCarriesItsFormalTwin(t *testing.T) {
	g, err := promote.ParseGlobal(formalGlobal)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}
	if len(g.Constrains) != 1 || g.Constrains[0].Formal != "額 % UNIT == 0" {
		t.Fatalf("promote.ParseGlobal() constrains = %+v, want one with its formal twin", g.Constrains)
	}

	x, diags := promote.Expand(g, promote.MapLoader(map[string]string{"local/ACCOUNT.puml": formalLocal}), promote.DefaultTemplates())
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}
	var clauses []promote.ExpandedClause
	for _, e := range x.Edges {
		for _, c := range e.Guard {
			if c.Kind == promote.ClauseConstrain {
				clauses = append(clauses, c)
			}
		}
	}
	want := []promote.ExpandedClause{{Kind: promote.ClauseConstrain, Text: "額 は最小単位の倍数", Formal: "額 % UNIT == 0", Line: 13}}
	if diff := cmp.Diff(want, clauses); diff != "" {
		t.Errorf("constrain clauses mismatch (-want +got):\n%s", diff)
	}
}

func TestParseGlobalRefusesAFormalLineThatBelongsToNothing(t *testing.T) {
	cases := map[string]string{
		"under a sync": strings.Replace(formalGlobal, "constrain DEPOSIT(口座ID, 額) ; 額 は最小単位の倍数", "sync DEPOSIT : accounts(口座ID), accounts(口座ID)", 1),
		"twice":        strings.Replace(formalGlobal, "  formal: 額 % UNIT == 0\n", "  formal: 額 % UNIT == 0\n  formal: True\n", 1),
		"empty":        strings.Replace(formalGlobal, "formal: 額 % UNIT == 0", "formal:", 1),
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := promote.ParseGlobal(source)
			if err == nil || !strings.Contains(err.Error(), "formal") {
				t.Fatalf("promote.ParseGlobal() error = %v, want a refusal naming formal", err)
			}
		})
	}
}
