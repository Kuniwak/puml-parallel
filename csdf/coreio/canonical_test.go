package coreio

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

func TestCanonicalOrdersStatesAndEdgesAsTheOldToolsPrinted(t *testing.T) {
	// Setup
	d := &core.Diagram{
		States: []string{"b", "a"},
		Init:   "a",
		Edges: []core.Edge{
			{Src: "b", Dst: "a", Event: core.Event{Channel: "x"}},
			{Src: "a", Dst: "b", Event: core.Event{Channel: "y"}},
			{Src: "a", Dst: "b", Event: core.Event{Channel: "x"}, Guard: mustParse(t, "$v < 2")},
			{Src: "a", Dst: "b", Event: core.Event{Channel: "x"}, Guard: mustParse(t, "$v < 1")},
		},
	}
	wantStates := []string{"a", "b"}
	wantEdges := []core.Edge{d.Edges[3], d.Edges[2], d.Edges[1], d.Edges[0]}

	// Execute
	got, err := Canonical(d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantStates, got.States); diff != "" {
		t.Errorf("States (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantEdges, got.Edges, sameClauses); diff != "" {
		t.Errorf("Edges (-want +got):\n%s", diff)
	}
}

func TestWriteCanonical(t *testing.T) {
	cases := []struct {
		name    string
		d       *core.Diagram
		want    string
		wantErr string
	}{
		{
			name: "states and edges written in the canonical order",
			d: &core.Diagram{
				States: []string{"b", "a"},
				Init:   "a",
				Edges: []core.Edge{
					{Src: "b", Dst: "a", Event: core.Event{Channel: "x"}},
					{Src: "a", Dst: "b", Event: core.Event{Channel: "y"}},
				},
			},
			want: `@startuml
state "a" as a
state "b" as b
[*] --> a
a --> b : y
b --> a : x
@enduml
`,
		},
		{
			name: "two end edges, which Write refuses",
			d: &core.Diagram{States: []string{"a"}, Init: "a",
				Ends: []core.End{{Src: "a"}, {Src: "a"}}},
			wantErr: "one end edge",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got, err := WriteCanonical(c.d)

			// Assert
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("WriteCanonical: got %v, want an error containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("WriteCanonical (-want +got):\n%s", diff)
			}
		})
	}
}
