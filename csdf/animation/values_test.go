package animation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func jsonNumber(s string) json.Number { return json.Number(s) }

// promotedDiagram is a diagram a promotion made: its state holds the map
// accounts, whose instances are at opened, holding balance, or at closed.
func promotedDiagram() *core.Diagram {
	return &core.Diagram{
		States: []string{"G"},
		Vars:   map[string][]string{"G": {"accounts", "total"}},
		Init:   "G",
		Families: []core.FamilyStates{{
			Map: "accounts", Init: "none", States: []string{"none", "opened", "closed"},
			Vars: map[string][]string{"opened": {"balance"}},
		}},
		InitPost: clause.And{},
	}
}

func TestEnterValuesTakesInstancesOfAFamily(t *testing.T) {
	// Setup
	session, err := NewSession(promotedDiagram(), NoSuggester{})
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	state, err := session.EnterValues(`[{"a": {"at": "opened", "fields": {"balance": 3}}, "b": {"at": "closed", "fields": {}}}, 3]`)

	// Assert
	if err != nil {
		t.Fatalf("EnterValues() error = %v", err)
	}
	want := State{ID: "G", Name: "G", Values: []Value{
		{Name: "accounts", Value: map[string]any{
			"a": map[string]any{"at": "opened", "fields": map[string]any{"balance": jsonNumber("3")}},
			"b": map[string]any{"at": "closed", "fields": map[string]any{}},
		}},
		{Name: "total", Value: jsonNumber("3")},
	}}
	if diff := cmp.Diff(want, state); diff != "" {
		t.Errorf("EnterValues() mismatch (-want +got):\n%s", diff)
	}
}

func TestEnterValuesRefusesAnInstanceThatIsNoneOfTheFamilys(t *testing.T) {
	tests := map[string]struct {
		accounts string
		want     string
	}{
		"not a map": {
			accounts: `[]`,
			want:     `accounts: a map of instances is a JSON object from IDs to {"at": <state>, "fields": {...}}`,
		},
		"an instance of no at": {
			accounts: `{"a": {"fields": {}}}`,
			want:     `accounts["a"]: an instance is {"at": <state>, "fields": {...}}`,
		},
		"an instance of another key": {
			accounts: `{"a": {"at": "closed", "fields": {}, "x": 1}}`,
			want:     `accounts["a"]: an instance is {"at": <state>, "fields": {...}}`,
		},
		"a state of no family": {
			accounts: `{"a": {"at": "frozen", "fields": {}}}`,
			want:     `accounts["a"]: "frozen" is no state of the family of accounts; it is at one of [opened, closed]`,
		},
		"the start state, which is absence": {
			accounts: `{"a": {"at": "none", "fields": {}}}`,
			want:     `accounts["a"]: "none" is the start state of the family of accounts, which is absence; leave the entry out`,
		},
		"a field the state does not hold": {
			accounts: `{"a": {"at": "opened", "fields": {"balance": 1, "owner": "x"}}}`,
			want:     `accounts["a"]: an instance at opened holds the fields [balance], got [balance, owner]`,
		},
		"a field left out": {
			accounts: `{"a": {"at": "opened", "fields": {}}}`,
			want:     `accounts["a"]: an instance at opened holds the fields [balance], got []`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			session, err := NewSession(promotedDiagram(), NoSuggester{})
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			_, err = session.EnterValues(`[` + tt.accounts + `, 0]`)

			// Assert
			var refused *RefusedValuesError
			if !errors.As(err, &refused) || refused.Error() != tt.want {
				t.Fatalf("EnterValues() error = %v, want refused values saying %q", err, tt.want)
			}
		})
	}
}
