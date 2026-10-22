package coreio

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/core"
)

func TestWriteWritesTheSyntaxReadReads(t *testing.T) {
	cases := []struct {
		name string
		d    func(t *testing.T) *core.Diagram
		want string
	}{
		{
			name: "states, variables, the start edge and edges",
			d: func(t *testing.T) *core.Diagram {
				return &core.Diagram{
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
				}
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
			d: func(t *testing.T) *core.Diagram {
				return &core.Diagram{
					States: []string{"a", "b"},
					Init:   "a",
					Edges: []core.Edge{
						{Src: "a", Dst: "b", Event: core.Event{Channel: "put", Fields: []core.Field{{Name: "k"}}},
							Guard: mustParse(t, "@exists(y : Y) ($y < $k)")},
					},
					Ends: []core.End{{Src: "b", Guard: mustParse(t, "片付いている")}},
				}
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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			d := c.d(t)

			// Execute
			got, err := Write(d)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("Write (-want +got):\n%s", diff)
			}
			back, err := Read(got)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(normalised(d), back, sameClauses); diff != "" {
				t.Errorf("Read(Write(d)) (-d +got):\n%s", diff)
			}
		})
	}
}

// normalised is d as Read gives it back: every state in Vars.
func normalised(d *core.Diagram) *core.Diagram {
	out := *d
	if out.Vars == nil {
		out.Vars = map[string][]string{}
	}
	return &out
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
