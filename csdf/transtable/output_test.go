package transtable_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
)

func TestParseOutput(t *testing.T) {
	type testCase struct {
		Name    string
		Want    transtable.Output
		WantErr bool
	}

	testCases := map[string]testCase{
		"tsv":     {Name: transtable.OutputNameTSV, Want: transtable.OutputTSV},
		"json":    {Name: transtable.OutputNameJSON, Want: transtable.OutputJSON},
		"unknown": {Name: "csv", WantErr: true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			got, err := transtable.ParseOutput(testCase.Name)

			// Assert
			if (err != nil) != testCase.WantErr {
				t.Fatalf("want an error %t, got %v", testCase.WantErr, err)
			}
			if got != testCase.Want {
				t.Errorf("want %v, got %v", testCase.Want, got)
			}
		})
	}
}

// Write writes what the writer of the output writes, and a notation spells
// the conditions of TSV only: JSON has syntax trees.
func TestWrite(t *testing.T) {
	type testCase struct {
		Output        transtable.Output
		Notation      logic.Notation
		WantSpells    bool
		WantInWritten string
	}

	table := &transtable.Table{
		Columns: []transtable.Column{transtable.EventColumn{Event: "a"}},
		Rows:    []transtable.Row{{State: "s0", Name: "S0", Cells: [][]transtable.Outcome{{refuse(not(q("g", c, x)))}}}},
	}

	testCases := map[string]testCase{
		"tsv, in the notation given": {
			Output:        transtable.OutputTSV,
			Notation:      format(transtable.NotationLogical).Notation,
			WantSpells:    true,
			WantInWritten: `¬""g""(c, x)`,
		},
		"json, which needs no notation": {
			Output:        transtable.OutputJSON,
			WantInWritten: `{"op":"not"`,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var sb strings.Builder

			// Act
			err := transtable.Write(&sb, table, testCase.Output, testCase.Notation)

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if !strings.Contains(sb.String(), testCase.WantInWritten) {
				t.Errorf("want %q in what is written, got %q", testCase.WantInWritten, sb.String())
			}
			if diff := cmp.Diff(testCase.WantSpells, testCase.Output.SpellsConditions()); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestWriteRefusesAnOutputItDoesNotKnow(t *testing.T) {
	// Arrange
	var sb strings.Builder

	// Act
	err := transtable.Write(&sb, &transtable.Table{}, 0, logic.Notation{})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "unknown output") {
		t.Errorf("want an unknown output error, got %v", err)
	}
}
