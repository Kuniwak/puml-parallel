package transtable_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
)

func TestWriteTSV(t *testing.T) {
	type testCase struct {
		Diagram string
		Format  transtable.Format
		Want    string
	}

	testCases := map[string]testCase{
		"a state and an event name a row and a column": {
			Diagram: `@startuml
state "Zero" as s0
state "One" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationNatural},
			Want: "state\tname\ta\n" +
				"s0\tZero\t→ s1\n" +
				"s1\tOne\t×\n",
		},
		"a condition comes first in brackets, and a cell holds a line per outcome": {
			Diagram: `@startuml
state "S0" as s0
state "S1" as s1
[*] --> s0
s0 --> s1 : a ; product is available
s1 --> [*] : g
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationNatural},
			Want: "state\tname\ta\t[*]\n" +
				"s0\tS0\t\"[product is available] → s1\n[not (product is available)] ×\"\t×\n" +
				"s1\tS1\t×\t\"[g] → [*]\n[not (g)] ×\"\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			table, err := transtable.Build(parse(t, testCase.Diagram))
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			var sb strings.Builder

			// Act
			err = transtable.WriteTSV(&sb, table, testCase.Format)

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
