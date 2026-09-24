package transtable_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
	"github.com/google/go-cmp/cmp"
)

func TestWriteTSV(t *testing.T) {
	type testCase struct {
		Diagram string
		Options transtable.Options
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
			Options: transtable.Options{Posts: true},
			Format:  transtable.Format{Notation: transtable.NotationNatural},
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
			Options: transtable.Options{Posts: true},
			Format:  transtable.Format{Notation: transtable.NotationLogical},
			Want: "state\tname\ta\tb\n" +
				"A\tA\t/ true ⨾ y' = 0 → C\t×\n" +
				"B\tB\t/ y' = 0 → C\t×\n" +
				"C\tC\t×\t→ A\n",
		},
		// The two outcomes of A are taken in B and in C, which the table does
		// not show, so they are one line.
		"paths that differ only where the table does not look are one line": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
state "E" as E
[*] --> A
A --> B : tau
A --> C : tau
B --> E : a
C --> E : a
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationNatural},
			Want: "state\tname\ta\n" +
				"A\tA\t→ E\n" +
				"B\tB\t→ E\n" +
				"C\tC\t→ E\n" +
				"E\tE\t×\n",
		},
		// Postconditions all true say nothing, so paths of different lengths
		// may come out the same even when postconditions are asked for.
		"outcomes whose postconditions say nothing are one line": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "E" as E
[*] --> A
A --> B : tau
A --> E : a
B --> E : a
@enduml
`,
			Options: transtable.Options{Posts: true},
			Format:  transtable.Format{Notation: transtable.NotationNatural},
			Want: "state\tname\ta\n" +
				"A\tA\t→ E\n" +
				"E\tE\t×\n" +
				"B\tB\t→ E\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			table, err := transtable.Build(csdf.MustParse(testCase.Diagram), testCase.Options)
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

// A reader finds a column by its header, so an event spelled like a fixed
// column would be read as that column.
func TestWriteTSVRefusesAnEventSpelledLikeAFixedColumn(t *testing.T) {
	for _, event := range []string{"state", "name", "[*]"} {
		t.Run(event, func(t *testing.T) {
			// Arrange
			table, err := transtable.Build(csdf.MustParse(`@startuml
state "S0" as s0
[*] --> s0
s0 --> s0 : `+event+`
@enduml
`), transtable.Options{})
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			var sb strings.Builder

			// Act
			err = transtable.WriteTSV(&sb, table, transtable.Format{Notation: transtable.NotationNatural})

			// Assert
			if err == nil {
				t.Fatalf("want an error, got the table %q", sb.String())
			}
			if !strings.Contains(err.Error(), event) {
				t.Errorf("want the event %q in the message, got %q", event, err.Error())
			}
			if sb.Len() != 0 {
				t.Errorf("want nothing written, got %q", sb.String())
			}
		})
	}
}

// A notation that spells a connective as nothing would print a negated guard
// as the guard itself, so it is refused before anything is written.
func TestWriteTSVRefusesANotationMissingAConnective(t *testing.T) {
	testCases := map[string]transtable.Notation{
		"the zero notation": {},
		"no and":            {Not: "not ", Then: " then "},
		"no not":            {And: " and ", Then: " then "},
		"no then":           {And: " and ", Not: "not "},
	}

	for name, notation := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			table, err := transtable.Build(csdf.MustParse(`@startuml
state "S0" as s0
state "S1" as s1
[*] --> s0
s0 --> s1 : a ; g
@enduml
`), transtable.Options{})
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			var sb strings.Builder

			// Act
			err = transtable.WriteTSV(&sb, table, transtable.Format{Notation: notation})

			// Assert
			if err == nil {
				t.Fatalf("want an error, got the table %q", sb.String())
			}
			if sb.Len() != 0 {
				t.Errorf("want nothing written, got %q", sb.String())
			}
		})
	}
}
