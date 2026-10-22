package coreio

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// written are diagrams with the text Write writes of them.
func written(t *testing.T) []struct {
	name string
	d    *core.Diagram
	want string
} {
	return []struct {
		name string
		d    *core.Diagram
		want string
	}{
		{
			name: "states, variables, the start edge and edges",
			d: &core.Diagram{
				States:   []string{"s"},
				Vars:     map[string][]string{"s": {"x"}},
				Init:     "s",
				InitPost: mustParse(t, "$x' = 0"),
				Edges: []core.Edge{
					{Src: "s", Dst: "s", Event: core.Event{Channel: "inc", Fields: []core.Field{{Send: true, Name: "x"}}},
						Guard: mustParse(t, "$x < 1"), Post: mustParse(t, "$x' = $x + 1")},
					{Src: "s", Dst: "s", Event: core.Event{Channel: "reset"}, Post: mustParse(t, "$x' = 0")},
					{Src: "s", Dst: "s", Event: core.Event{Channel: "look"}, Guard: mustParse(t, "$x ; は 0")},
				},
			},
			want: `@startuml
state "s" as s
s : x
[*] --> s : $x' = 0
s --> s : inc!x ; $x < 1 ; $x' = $x + 1
s --> s : reset ; ; $x' = 0
s --> s : look ; $x \; は 0
@enduml
`,
		},
		{
			name: "∃, received fields and an end edge",
			d: &core.Diagram{
				States: []string{"a", "b"},
				Vars:   map[string][]string{},
				Init:   "a",
				Edges: []core.Edge{
					{Src: "a", Dst: "b", Event: core.Event{Channel: "put", Fields: []core.Field{{Name: "k"}}},
						Guard: mustParse(t, "@exists(y : Y) ($y < $k)")},
				},
				Ends: []core.End{{Src: "b", Guard: mustParse(t, "片付いている")}},
			},
			want: `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : put?k ; @exists(y : Y) ($y < $k)
b --> [*] : 片付いている
@enduml
`,
		},
		{
			name: "an assume note",
			d: &core.Diagram{
				States:  []string{"s"},
				Vars:    map[string][]string{},
				Init:    "s",
				Edges:   []core.Edge{{Src: "s", Dst: "s", Event: core.Event{Channel: "a"}}},
				Assumes: []core.Assumption{{ID: "fair", Text: "いずれ a が起きる"}},
			},
			want: `@startuml
state "s" as s
[*] --> s
s --> s : a
note as fair
  assume いずれ a が起きる
end note
@enduml
`,
		},
		{
			name: "a title and states labelled apart from their IDs",
			d: &core.Diagram{
				Name:   "auto-generated-by: csdfparallel a.puml b.puml",
				States: []string{"s0_s0", "s1_s0"},
				Labels: map[string]string{"s0_s0": "(s0, s0)", "s1_s0": "(s1, s0)"},
				Vars:   map[string][]string{},
				Init:   "s0_s0",
				Edges:  []core.Edge{{Src: "s0_s0", Dst: "s1_s0", Event: core.Event{Channel: "go"}}},
			},
			want: `@startuml auto-generated-by: csdfparallel a.puml b.puml
state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
[*] --> s0_s0
s0_s0 --> s1_s0 : go
@enduml
`,
		},
		{
			name: "states in an order of their own",
			d: &core.Diagram{
				States: []string{"z", "a"},
				Vars:   map[string][]string{"z": {"x"}},
				Init:   "a",
				Edges:  []core.Edge{{Src: "a", Dst: "z", Event: core.Event{Channel: "go"}}},
			},
			want: `@startuml
state "z" as z
z : x
state "a" as a
[*] --> a
a --> z : go
@enduml
`,
		},
	}
}

func TestWrite(t *testing.T) {
	for _, c := range written(t) {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got, err := Write(c.d)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("Write (-want +got):\n%s", diff)
			}
		})
	}
}

// TestReadOfWriteIsTheDiagram reads back what Write wrote: Read keeps the
// order the states are written in, and gives Vars even when it is empty.
func TestReadOfWriteIsTheDiagram(t *testing.T) {
	for _, c := range written(t) {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			text := c.want

			// Execute
			got, err := Read(text)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.d, got, sameClauses); diff != "" {
				t.Errorf("Read(Write(d)) (-d +got):\n%s", diff)
			}
		})
	}
}

func TestWriteRefuses(t *testing.T) {
	cases := []struct {
		name string
		d    *core.Diagram
		want string
	}{
		{
			name: "two end edges, which the syntax cannot write yet",
			d: &core.Diagram{States: []string{"a"}, Init: "a",
				Ends: []core.End{{Src: "a"}, {Src: "a"}}},
			want: "one end edge",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, err := Write(c.d)

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Write: got %v, want an error containing %q", err, c.want)
			}
		})
	}
}
