package transtable_test

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
)

// A condition over a tau step, to spell in either notation.
var afterTau = ex(vs(c1, x1), and(q("h", c1, x), not(q("g", c, x1))))

func TestWriteTSV(t *testing.T) {
	type testCase struct {
		Table  *transtable.Table
		Format transtable.Format
		Want   string
	}

	testCases := map[string]testCase{
		"a state and an event name a row and a column": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{
					{State: "s0", Name: "Zero", Cells: [][]transtable.Outcome{{goTo(logic.True, "s1")}}},
					{State: "s1", Name: "One", Cells: [][]transtable.Outcome{{refuse(logic.True)}}},
				},
			},
			Format: format(transtable.NotationNatural),
			Want: "state\tname\ta\n" +
				"s0\tZero\t→ s1\n" +
				"s1\tOne\t×\n",
		},
		// Predicates are quoted, and csv.Writer doubles the quotes of a cell
		// holding them, as CSV does.
		"a condition comes first in brackets, and a cell holds a line per outcome": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}, transtable.TerminationColumn{}},
				Rows: []transtable.Row{
					{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{
						{goTo(q("product is available", c, x), "s1"), refuse(not(q("product is available", c, x)))},
						{refuse(logic.True)},
					}},
					{State: "s1", Name: "S1", Cells: [][]transtable.Outcome{
						{refuse(logic.True)},
						{{Cond: q("g", x), Result: transtable.Terminate{}}, refuse(not(q("g", x)))},
					}},
				},
			},
			Format: format(transtable.NotationNatural),
			Want: "state\tname\ta\t[*]\n" +
				"s0\tS0\t\"[\"\"product is available\"\"(c, x)] → s1\n[not \"\"product is available\"\"(c, x)] ×\"\t×\n" +
				"s1\tS1\t×\t\"[\"\"g\"\"(x)] → [*]\n[not \"\"g\"\"(x)] ×\"\n",
		},
		"the natural notation spells the connectives as words": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows:    []transtable.Row{{State: "A", Name: "A", Cells: [][]transtable.Outcome{{refuse(afterTau)}}}},
			},
			Format: format(transtable.NotationNatural),
			Want: tsvOf(
				[]string{"state", "name", "a"},
				[]string{"A", "A", `[exists c1 x1. "h"(c1, x) and not "g"(c, x1)] ×`},
			),
		},
		"the logical notation spells them as symbols": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows:    []transtable.Row{{State: "A", Name: "A", Cells: [][]transtable.Outcome{{refuse(afterTau)}}}},
			},
			Format: format(transtable.NotationLogical),
			Want: tsvOf(
				[]string{"state", "name", "a"},
				[]string{"A", "A", `[∃c1 x1. "h"(c1, x) ∧ ¬"g"(c, x1)] ×`},
			),
		},
		// The table does not show the states a tau path passes, so two
		// outcomes may come out the same.
		"a line already written in a cell is not written again": {
			Table: &transtable.Table{
				Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
				Rows: []transtable.Row{{State: "A", Name: "A", Cells: [][]transtable.Outcome{{
					goTo(logic.True, "E"), refuse(q("g", c, x)), goTo(logic.True, "E"),
				}}}},
			},
			Format: format(transtable.NotationLogical),
			Want: tsvOf(
				[]string{"state", "name", "a"},
				[]string{"A", "A", lines("→ E", `["g"(c, x)] ×`)},
			),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var sb strings.Builder

			// Act
			err := transtable.WriteTSV(&sb, testCase.Table, testCase.Format)

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

// Nothing is written when the table cannot be: a reader finds a column by its
// header, so an event spelled like a fixed column would be read as that one,
// and the zero notation spells nothing, which would print a negated guard as
// the guard itself.
func TestWriteTSVRefuses(t *testing.T) {
	type testCase struct {
		Table       *transtable.Table
		Format      transtable.Format
		WantInError string
	}

	eventNamed := func(event string) *transtable.Table {
		return &transtable.Table{
			Columns: []transtable.Column{transtable.EventColumn{Event: "a"}, transtable.EventColumn{Event: csdf.Event(event)}},
			Rows:    []transtable.Row{{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{refuse(logic.True)}, {refuse(logic.True)}}}},
		}
	}

	testCases := map[string]testCase{
		"an event spelled state": {Table: eventNamed("state"), Format: format(transtable.NotationNatural), WantInError: `"state"`},
		"an event spelled name":  {Table: eventNamed("name"), Format: format(transtable.NotationNatural), WantInError: `"name"`},
		"an event spelled [*]":   {Table: eventNamed("[*]"), Format: format(transtable.NotationNatural), WantInError: `"[*]"`},
		"the zero notation":      {Table: eventNamed("b"), Format: transtable.Format{}, WantInError: "zero notation"},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var sb strings.Builder

			// Act
			err := transtable.WriteTSV(&sb, testCase.Table, testCase.Format)

			// Assert
			if err == nil {
				t.Fatalf("want an error, got the table %q", sb.String())
			}
			if !strings.Contains(err.Error(), testCase.WantInError) {
				t.Errorf("want %q in the message, got %q", testCase.WantInError, err.Error())
			}
			if sb.Len() != 0 {
				t.Errorf("want nothing written, got %q", sb.String())
			}
		})
	}
}

func TestParseNotation(t *testing.T) {
	type testCase struct {
		Name    string
		WantErr bool
	}

	testCases := map[string]testCase{
		"natural": {Name: transtable.NotationNatural},
		"logical": {Name: transtable.NotationLogical},
		"unknown": {Name: "bogus", WantErr: true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := transtable.ParseNotation(testCase.Name)

			// Assert
			if (err != nil) != testCase.WantErr {
				t.Errorf("want an error %t, got %v", testCase.WantErr, err)
			}
		})
	}
}

// tsvOf is the TSV WriteTSV writes for records, quoted as csv.Writer quotes.
func tsvOf(records ...[]string) string {
	var sb strings.Builder
	cw := csv.NewWriter(&sb)
	cw.Comma = '\t'
	if err := cw.WriteAll(records); err != nil {
		panic(err)
	}
	return sb.String()
}

// lines is a cell holding one outcome per line.
func lines(ls ...string) string { return strings.Join(ls, "\n") }

// format is the Format of the notation named name.
func format(name string) transtable.Format {
	n, err := transtable.ParseNotation(name)
	if err != nil {
		panic(err)
	}
	return transtable.Format{Notation: n}
}
