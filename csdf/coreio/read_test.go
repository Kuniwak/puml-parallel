package coreio

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

func mustParse(t *testing.T, text string) clause.And {
	t.Helper()
	f, err := clause.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// sameClauses compares clauses up to α-equivalence.
// sameClauses compares diagrams by their meaning: clauses up to the names of
// what they bind, and edges whatever line they were read from.
var sameClauses = cmp.Options{
	cmp.Comparer(func(a, b clause.And) bool { return clause.AlphaEqual(a, b) }),
	cmpopts.IgnoreFields(core.Edge{}, "Line"),
	cmpopts.IgnoreFields(core.Diagram{}, "InitLine"),
}

func TestReadKeepsTheLineOfEachEdge(t *testing.T) {
	// Execute
	d, err := Read("@startuml\nstate \"a\" as a\n[*] --> a\na --> a : x\n\na --> a : y\n@enduml\n")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got := []int{d.InitLine, d.Edges[0].Line, d.Edges[1].Line}; got[0] != 3 || got[1] != 4 || got[2] != 6 {
		t.Errorf("lines = %v, want [3 4 6]", got)
	}
}

func TestReadAuthoredReadsADiagram(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   func(t *testing.T) *core.Diagram
	}{
		{
			name: "states, variables, the start edge and edges",
			source: `@startuml
state "数えている" as s
s : x
[*] --> s : $x' = 0
s --> s : inc!x ; $x < 1 ; $x' = $x + 1
s --> s : reset ; ; $x' = 0
@enduml
`,
			want: func(t *testing.T) *core.Diagram {
				return &core.Diagram{
					Labels:   map[string]string{"s": "数えている"},
					States:   []string{"s"},
					Vars:     map[string][]string{"s": {"x"}},
					Init:     "s",
					InitPost: mustParse(t, "$x' = 0"),
					Edges: []core.Edge{
						{Src: "s", Dst: "s", Event: core.Event{Channel: "inc", Fields: []core.Field{{Send: true, Name: "x"}}},
							Guard: mustParse(t, "$x < 1"), Post: mustParse(t, "$x' = $x + 1")},
						{Src: "s", Dst: "s", Event: core.Event{Channel: "reset"},
							Post: mustParse(t, "$x' = 0")},
					},
				}
			},
		},
		{
			name: "received fields, a builtin and an end edge",
			source: `@startuml
state "a" as a
a : m
state "b" as b
[*] --> a : @empty(m')
a --> b : put?k?v ; @notin(k, m) ; @update(m', m, k, v)
b --> [*] : 片付いている
@enduml
`,
			want: func(t *testing.T) *core.Diagram {
				return &core.Diagram{
					States:   []string{"a", "b"},
					Vars:     map[string][]string{"a": {"m"}},
					Init:     "a",
					InitPost: mustParse(t, "@empty(m')"),
					Edges: []core.Edge{
						{Src: "a", Dst: "b", Event: core.Event{Channel: "put", Fields: []core.Field{{Name: "k"}, {Name: "v"}}},
							Guard: mustParse(t, "@notin(k, m)"), Post: mustParse(t, "@update(m', m, k, v)")},
					},
					Ends: []core.End{{Src: "b", Guard: mustParse(t, "片付いている")}},
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			want := c.want(t)

			// Execute
			got, err := ReadAuthored(c.source)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got, sameClauses); diff != "" {
				t.Errorf("ReadAuthored (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadAuthoredRefuses(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{
			name: "a clause holding ∃, which only tools write",
			source: `@startuml
state "s" as s
[*] --> s
s --> s : a ; @exists(y : Y) ($y が大きい)
@enduml
`,
			want: "written by tools only",
		},
		{
			name: "a tool word of the old vocabulary",
			source: `@startuml
state "s" as s
[*] --> s
s --> s : a ; @opaque
@enduml
`,
			want: "@opaque",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, err := ReadAuthored(c.source)

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("ReadAuthored: got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestReadReadsWhatToolsWrite(t *testing.T) {
	// Setup
	source := `@startuml
state "s" as s
[*] --> s
s --> s : a ; @exists(y : Y) ($y が大きい)
@enduml
`
	want := mustParse(t, "@exists(y : Y) ($y が大きい)")

	// Execute
	got, err := Read(source)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !clause.AlphaEqual(got.Edges[0].Guard, want) {
		t.Errorf("Read: the guard is %v, want %v", got.Edges[0].Guard, want)
	}
}

func TestReadHintsAtPromotionForAGlobalDiagram(t *testing.T) {
	// Setup
	src, err := os.ReadFile("../../examples/promote/TRADES.puml")
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err = Read(string(src))

	// Assert
	var hinted *csdf.PromotionHintError
	if !errors.As(err, &hinted) {
		t.Errorf("err = %v, want a *csdf.PromotionHintError", err)
	}
}

func TestReadRefusesAFieldReceivedOverAStateVariable(t *testing.T) {
	testCases := map[string]string{
		"of the source":      "@startuml\nstate \"a\" as a\na : k\nstate \"b\" as b\n[*] --> a\na --> b : in?k\n@enduml\n",
		"of the destination": "@startuml\nstate \"a\" as a\nstate \"b\" as b\nb : k\n[*] --> a\na --> b : in?k\n@enduml\n",
	}
	for name, src := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Read(src)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "k") {
				t.Errorf("err = %v, want the field k refused", err)
			}
		})
	}
}
