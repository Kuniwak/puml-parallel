package coreio

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/Kuniwak/puml-parallel/v3/csdf"
	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
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
s : x ; N
[*] --> s : $x' = 0
s --> s : inc!x ; $x < 1 ; $x' = $x + 1
s --> s : reset ; ; $x' = 0
note as channels
  channel inc : N
end note
@enduml
`,
			want: func(t *testing.T) *core.Diagram {
				return &core.Diagram{
					Labels:   map[string]string{"s": "数えている"},
					States:   []string{"s"},
					Vars:     map[string][]string{"s": {"x"}},
					Types:    map[string]types.Type{"x": types.Named("N")},
					Channels: map[string][]types.Type{"inc": {types.Named("N")}},
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
a : m ; K ⇸ V
state "b" as b
b : m ; K ⇸ V
[*] --> a : @empty(m')
a --> b : put?k?v ; @notin(k, m) ; @update(m', m, k, v)
b --> [*] : 片付いている
note as channels
  channel put : K.V
end note
@enduml
`,
			want: func(t *testing.T) *core.Diagram {
				return &core.Diagram{
					States:   []string{"a", "b"},
					Vars:     map[string][]string{"a": {"m"}, "b": {"m"}},
					Types:    map[string]types.Type{"m": types.MapOf(types.Named("K"), types.Named("V"))},
					Channels: map[string][]types.Type{"put": {types.Named("K"), types.Named("V")}},
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

func TestReadAuthoredReadsAnExistsAnAuthorWrote(t *testing.T) {
	// Setup
	source := `@startuml
state "s" as s
s : n ; N
[*] --> s : @exists(x : N) (@eq(n', x))
s --> s : a ; ; @exists(y : N) (@eq(n', y))
@enduml
`
	want := mustParse(t, "@exists(y : N) (@eq(n', y))")

	// Execute
	got, err := ReadAuthored(source)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got.Edges[0].Post, sameClauses); diff != "" {
		t.Errorf("ReadAuthored post (-want +got):\n%s", diff)
	}
}

func TestReadAuthoredRefuses(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{
			name: "an ∃ binding a state variable of the source",
			source: `@startuml
state "s" as s
s : n ; N
state "t" as t
t : m ; N
[*] --> s : @eq(n', 0)
s --> t : a ; ; @exists(n : N) (@eq(m', n))
@enduml
`,
			want: "@exists binds n, which is a state variable of s; give it another name",
		},
		{
			name: "an ∃ binding a state variable of the target",
			source: `@startuml
state "s" as s
s : n ; N
state "t" as t
t : m ; N
[*] --> s : @eq(n', 0)
s --> t : a ; ; @exists(m : N) (@eq(n, m))
@enduml
`,
			want: "@exists binds m, which is a state variable of t; give it another name",
		},
		{
			name: "an ∃ binding a field of the event",
			source: `@startuml
state "s" as s
s : n ; N
state "t" as t
t : m ; N
[*] --> s : @eq(n', 0)
s --> t : c?k ; ; @exists(k : N) (@eq(m', k))
note as channels
  channel c : N
end note
@enduml
`,
			want: "@exists binds k, which is a field of the event; give it another name",
		},
		{
			name: "an ∃ binding an environment function",
			source: `@startuml
state "s" as s
s : n ; N
state "t" as t
t : m ; N
[*] --> s : @eq(n', 0)
s --> t : a ; ; @exists(f : N) (@eq(m', f(n)))
@enduml
`,
			want: "@exists binds f, which is an environment function; give it another name",
		},
		{
			name: "an ∃ binding a state variable on the start edge",
			source: `@startuml
state "s" as s
s : n ; N
[*] --> s : @exists(n : N) (@eq(n, 0))
@enduml
`,
			want: "@exists binds n, which is a state variable of s; give it another name",
		},
		{
			name: "an ∃ binding a state variable on the end edge",
			source: `@startuml
state "s" as s
s : n ; N
[*] --> s : @eq(n', 0)
s --> [*] : @exists(n : N) (@eq(n, 0))
@enduml
`,
			want: "the end edge from s: @exists binds n, which is a state variable of s; give it another name",
		},
		{
			name: "an edge whose ∃ binds a field, named by its line and event",
			source: `@startuml
state "s" as s
s : n ; N
[*] --> s : @eq(n', 0)
s --> s : c?k ; @exists(k : N) (@eq(n, k))
note as channels
  channel c : N
end note
@enduml
`,
			want: "line 5: s --> s : c?k: @exists binds k",
		},
		{
			name: "an ∃ binding a reserved word",
			source: `@startuml
state "s" as s
s : n ; N
state "t" as t
t : m ; N
[*] --> s : @eq(n', 0)
s --> t : a ; ; @exists(true : N) (@eq(m', 0))
@enduml
`,
			want: "true is a reserved word",
		},
		{
			name: "an ∃ shadowing an enclosing one",
			source: `@startuml
state "s" as s
s : n ; N
state "t" as t
t : m ; N
[*] --> s : @eq(n', 0)
s --> t : a ; ; @exists(x : N) (@exists(x : N) (@eq(m', x)))
@enduml
`,
			want: "x is bound by an enclosing @exists",
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
		{
			name: "a name nothing binds, such as a literal written as a name",
			source: `@startuml
state "s" as s
s : n ; n
[*] --> s : @eq(n', 0)
s --> s : a ; @eq(n, zero)
@enduml
`,
			want: "the guard reads zero, which is no state variable of s, no field of the event and bound by no ∃",
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
	src, err := os.ReadFile("../../examples/promote/RECEIPTS.puml")
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
		"of the source":      "@startuml\nstate \"a\" as a\na : k ; k\nstate \"b\" as b\n[*] --> a\na --> b : in?k\nnote as channels\n  channel in : k\nend note\n@enduml\n",
		"of the destination": "@startuml\nstate \"a\" as a\nstate \"b\" as b\nb : k ; k\n[*] --> a\na --> b : in?k\nnote as channels\n  channel in : k\nend note\n@enduml\n",
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

func TestReadTagsARefusedClauseWithTheLineOfItsEdge(t *testing.T) {
	// Execute: the clause on the fourth line is the old @opaque, which the
	// formula reader refuses after the parser has read past that line.
	_, err := Read("@startuml\nstate \"s0\" as s0\n[*] --> s0\ns0 --> s0 : a ; @opaque\n\n@enduml\n")

	// Assert
	var parseErr *csdf.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("want a *csdf.ParseError, got %v", err)
	}
	if parseErr.Line != 4 {
		t.Errorf("line = %d, want 4", parseErr.Line)
	}
}
