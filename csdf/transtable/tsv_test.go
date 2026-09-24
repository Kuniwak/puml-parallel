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
		"the logical notation spells the connectives as symbols": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : tau ; h
B --> C : a ; g
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationLogical},
			Want: "state\tname\ta\n" +
				"A\tA\t\"[¬(h)] ×\n[h ∧ g] → C\n[h ∧ ¬(g)] ×\"\n" +
				"B\tB\t\"[g] → C\n[¬(g)] ×\"\n" +
				"C\tC\t×\n",
		},
		"postconditions along the path are composed in order after the condition": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : tau ; h ; x' = x + 1
B --> C : a ; g ; y' = x
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationNatural, Posts: true},
			Want: "state\tname\ta\n" +
				"A\tA\t\"[not (h)] ×\n[h and g] / x' = x + 1 then y' = x → C\n[h and not (g)] / x' = x + 1 ×\"\n" +
				"B\tB\t\"[g] / y' = x → C\n[not (g)] ×\"\n" +
				"C\tC\t×\n",
		},
		"postconditions are left out when every one along the path is true, and kept otherwise": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : tau
B --> C : a ; true ; y' = 0
C --> A : b
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationLogical, Posts: true},
			Want: "state\tname\ta\tb\n" +
				"A\tA\t/ true ⨾ y' = 0 → C\t×\n" +
				"B\tB\t/ y' = 0 → C\t×\n" +
				"C\tC\t×\t→ A\n",
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
