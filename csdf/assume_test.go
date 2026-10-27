package csdf

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const assumeSource = `@startuml
state "s0" as s0
s0 : n ; n
[*] --> s0 : n' = 0
s0 --> s0 : tau ; ; n' = n + 1
note as fair
  assume the run has finitely many tau steps
end note
@enduml
`

func TestParseReadsAnAssumeNote(t *testing.T) {
	d := mustParseAccepting(assumeSource)

	want := []Assumption{{ID: "fair", Text: "the run has finitely many tau steps", Line: 7}}
	if diff := cmp.Diff(want, d.Assumptions); diff != "" {
		t.Error(diff)
	}
}

func TestParseGivesAnEmptyAssumptionListRatherThanNull(t *testing.T) {
	bs, err := json.Marshal(mustParseAccepting("@startuml\n[*] --> s0\n@enduml\n"))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(bs), `"assumptions":[]`) {
		t.Errorf("want \"assumptions\":[] in the JSON, got %s", bs)
	}
}

func TestParseRejectsANoteThatIsNotAnAssumption(t *testing.T) {
	cases := map[string]string{
		"prose":                       "@startuml\n[*] --> s0\nnote as n\n  for the picture\nend note\n@enduml\n",
		"empty":                       "@startuml\n[*] --> s0\nnote as n\nend note\n@enduml\n",
		"anchored":                    "@startuml\nstate \"s0\" as s0\n[*] --> s0\nnote left of s0\n  assume x\nend note\n@enduml\n",
		"two assumptions in one note": "@startuml\n[*] --> s0\nnote as n\n  assume x\n  assume y\nend note\n@enduml\n",
		"unterminated":                "@startuml\n[*] --> s0\nnote as n\n  assume x\n@enduml\n",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseAccepting(source)
			if err == nil {
				t.Fatal("parseAccepting() error = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), "assume") {
				t.Errorf("parseAccepting() error = %v, want it to say what a note may hold", err)
			}
		})
	}
}

// The grammar lets an assume note follow the end edge, which is the last
// transition: an assumption is about the whole diagram, so it reads well last.
func TestParseReadsAnAssumeNoteAfterTheEndEdge(t *testing.T) {
	d := mustParseAccepting("@startuml\n[*] --> s0\ns0 --> [*]\nnote as fair\n  assume x\nend note\n@enduml\n")
	if len(d.Assumptions) != 1 || d.Assumptions[0].ID != "fair" {
		t.Errorf("Assumptions = %v, want the one note", d.Assumptions)
	}
	if _, err := parseAccepting("@startuml\n[*] --> s0\ns0 --> [*]\ns0 --> s0 : a\n@enduml\n"); err == nil {
		t.Error("parseAccepting() accepted an edge after the end edge")
	}
}
