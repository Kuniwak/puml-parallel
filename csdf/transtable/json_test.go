package transtable_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
)

func TestWriteJSON(t *testing.T) {
	type testCase struct {
		Table *transtable.Table
		Want  string
	}

	testCases := map[string]testCase{
		"an empty table has empty lists, not nulls": {
			Table: &transtable.Table{},
			Want:  `{"columns":[],"rows":[],"unreachable":[]}` + "\n",
		},
		"a column says what it is, and a cell lists its outcomes in the order of the columns": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}, transtable.TerminationColumn{}},
				Rows: []transtable.Row{{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{
					{goTo(q("g", c, x), "s1"), refuse(not(q("g", c, x)))},
					{{Cond: logic.True, Result: transtable.Terminate{}}},
				}}},
				Unreachable: []csdf.StateID{"z"},
			},
			Want: `{"columns":[{"kind":"event","event":"a"},{"kind":"termination"}],` +
				`"rows":[{"state":"s0","name":"S0","cells":[` +
				`[{"result":"goto","state":"s1","condition":{"op":"atom","name":"g","quoted":true,"args":["c","x"]}},` +
				`{"result":"refuse","condition":{"op":"not","operands":[{"op":"atom","name":"g","quoted":true,"args":["c","x"]}]}}],` +
				`[{"result":"terminate","condition":{"op":"true"}}]]}],` +
				`"unreachable":["z"]}` + "\n",
		},
		// Columns are told apart by their kind, not by their headers, so an
		// event spelled like a column of the TSV is no clash here.
		"an event spelled like a column of the TSV is an event": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "[*]"}},
				Rows:    []transtable.Row{{State: "s0", Name: "a <b> & c", Cells: [][]transtable.Outcome{{refuse(logic.True)}}}},
			},
			Want: `{"columns":[{"kind":"event","event":"[*]"}],` +
				`"rows":[{"state":"s0","name":"a <b> & c","cells":[[{"result":"refuse","condition":{"op":"true"}}]]}],` +
				`"unreachable":[]}` + "\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var sb strings.Builder

			// Act
			err := transtable.WriteJSON(&sb, testCase.Table)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if diff := cmp.Diff(testCase.Want, sb.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
