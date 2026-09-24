package transtable_test

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
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
		// Predicates are quoted, and csv.Writer doubles the quotes of a cell
		// holding them, as CSV does.
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
				"s0\tS0\t\"[\"\"product is available\"\"(c, x)] → s1\n[not \"\"product is available\"\"(c, x)] ×\"\t×\n" +
				"s1\tS1\t×\t\"[\"\"g\"\"(x)] → [*]\n[not \"\"g\"\"(x)] ×\"\n",
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
			Want: tsvOf(
				[]string{"state", "name", "a"},
				[]string{"A", "A", lines(
					`[¬(∃c1. "h"(c1, x))] ×`,
					`[∃c1 x1. "h"(c1, x) ∧ "g"(c, x1)] → C`,
					`[∃c1 x1. "h"(c1, x) ∧ ¬"g"(c, x1)] ×`,
				)},
				[]string{"B", "B", lines(`["g"(c, x)] → C`, `[¬"g"(c, x)] ×`)},
				[]string{"C", "C", "×"},
			),
		},
		"a postcondition is applied to the values before and after its step": {
			Diagram: `@startuml
state "A" as A
state "B" as B
state "C" as C
[*] --> A
A --> B : tau ; h ; x' = x + 1
B --> C : a ; g ; y' = x
@enduml
`,
			Format: transtable.Format{Notation: transtable.NotationNatural},
			Want: tsvOf(
				[]string{"state", "name", "a"},
				[]string{"A", "A", lines(
					`[not (exists c1. "h"(c1, x))] ×`,
					`[exists c1 x1. "h"(c1, x) and "x' = x + 1"(c1, x, x1) and "g"(c, x1) and "y' = x"(c, x1, x')] → C`,
					`[exists c1 x1. "h"(c1, x) and "x' = x + 1"(c1, x, x1) and not "g"(c, x1)] ×`,
				)},
				[]string{"B", "B", lines(`["g"(c, x) and "y' = x"(c, x, x')] → C`, `[not "g"(c, x)] ×`)},
				[]string{"C", "C", "×"},
			),
		},
		// A true postcondition leaves the values after it free, so a later
		// predicate reads values that are only bound.
		"true guards and postconditions add nothing, and the values they leave free stay bound": {
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
			Format: transtable.Format{Notation: transtable.NotationLogical},
			Want: tsvOf(
				[]string{"state", "name", "a", "b"},
				[]string{"A", "A", `[∃x1. "y' = 0"(c, x1, x')] → C`, "×"},
				[]string{"B", "B", `["y' = 0"(c, x, x')] → C`, "×"},
				[]string{"C", "C", "×", "→ A"},
			),
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
		// True guards and postconditions say nothing, so paths of different
		// lengths may come out the same.
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
			Format: transtable.Format{Notation: transtable.NotationNatural},
			Want: "state\tname\ta\n" +
				"A\tA\t→ E\n" +
				"E\tE\t×\n" +
				"B\tB\t→ E\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			table, err := transtable.Build(csdf.MustParse(testCase.Diagram))
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
s0 --> s0 : ` + event + `
@enduml
`))
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
		"no and":            {Not: "not ", Exists: "exists "},
		"no not":            {And: " and ", Exists: "exists "},
		"no exists":         {And: " and ", Not: "not "},
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
`))
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
