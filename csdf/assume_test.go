package csdf

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const assumeSource = `@startuml
state "s0" as s0
s0: n
[*] --> s0 : n' = 0
s0 --> s0 : tau ; ; n' = n + 1
note as fair
  assume the run has finitely many tau steps
end note
@enduml
`

func TestParseReadsAnAssumeNote(t *testing.T) {
	d := MustParse(assumeSource)

	want := []Assumption{{ID: "fair", Text: "the run has finitely many tau steps", Line: 7}}
	if diff := cmp.Diff(want, d.Assumptions); diff != "" {
		t.Error(diff)
	}
}

func TestParseGivesAnEmptyAssumptionListRatherThanNull(t *testing.T) {
	bs, err := json.Marshal(MustParse("@startuml\n[*] --> s0\n@enduml\n"))
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
			_, err := Parse(source)
			if err == nil {
				t.Fatal("Parse() error = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), "assume") {
				t.Errorf("Parse() error = %v, want it to say what a note may hold", err)
			}
		})
	}
}

func TestDiagramStringRoundTripsAssumptions(t *testing.T) {
	if diff := cmp.Diff(assumeSource, MustParse(assumeSource).String()); diff != "" {
		t.Error(diff)
	}
}

func TestCloneCopiesTheAssumptions(t *testing.T) {
	d := MustParse(assumeSource)
	c := d.Clone()
	c.Assumptions[0].Text = "changed"
	if d.Assumptions[0].Text == "changed" {
		t.Error("Clone() shares the assumption list with its input")
	}
}

func TestTransformationsKeepTheAssumptions(t *testing.T) {
	d := MustParse(assumeSource)
	want := d.Assumptions

	if got := Hide(d, []string{"a"}).Assumptions; !cmp.Equal(want, got) {
		t.Errorf("Hide() assumptions = %v, want %v", got, want)
	}
	sorted := Sort(d).Assumptions
	if len(sorted) != 1 || sorted[0].Line != 0 || sorted[0].ID != "fair" {
		t.Errorf("Sort() assumptions = %v, want the assumption with its line dropped", sorted)
	}
}

func TestComposeParallelConcatenatesTheAssumptions(t *testing.T) {
	other := MustParse("@startuml\n[*] --> t0\nt0 --> t0 : a\nnote as bound\n  assume a happens at most twice\nend note\n@enduml\n")
	got, err := ComposeParallel([]*Diagram{MustParse(assumeSource), other}, nil)
	if err != nil {
		t.Fatalf("ComposeParallel() error = %v", err)
	}
	want := []Assumption{
		{ID: "fair", Text: "the run has finitely many tau steps", Line: 7},
		{ID: "bound", Text: "a happens at most twice", Line: 5},
	}
	if diff := cmp.Diff(want, got.Assumptions); diff != "" {
		t.Error(diff)
	}
}

// The grammar lets an assume note follow the end edge, which is the last
// transition: an assumption is about the whole diagram, so it reads well last.
func TestParseReadsAnAssumeNoteAfterTheEndEdge(t *testing.T) {
	d := MustParse("@startuml\n[*] --> s0\ns0 --> [*]\nnote as fair\n  assume x\nend note\n@enduml\n")
	if len(d.Assumptions) != 1 || d.Assumptions[0].ID != "fair" {
		t.Errorf("Assumptions = %v, want the one note", d.Assumptions)
	}
	if _, err := Parse("@startuml\n[*] --> s0\ns0 --> [*]\ns0 --> s0 : a\n@enduml\n"); err == nil {
		t.Error("Parse() accepted an edge after the end edge")
	}
}
