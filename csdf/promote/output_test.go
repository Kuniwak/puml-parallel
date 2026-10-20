package promote_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

func TestParseOutput(t *testing.T) {
	cases := map[string]struct {
		name    string
		want    promote.Output
		wantErr bool
	}{
		"puml":    {name: promote.OutputNamePUML, want: promote.OutputPUML},
		"json":    {name: promote.OutputNameJSON, want: promote.OutputJSON},
		"unknown": {name: "tsv", wantErr: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := promote.ParseOutput(c.name)
			if (err != nil) != c.wantErr {
				t.Fatalf("want an error %t, got %v", c.wantErr, err)
			}
			if got != c.want {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

// The JSON output is the expansion as Expand returns it: the diagram, and every
// edge with its origin and its clauses, for a program that must not parse the
// wording back.
func TestWriteJSONPrintsTheExpansionWithItsClauses(t *testing.T) {
	x, diags := expand(t, syncGlobal, syncLocals)
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}

	var buf bytes.Buffer
	if err := promote.Write(&buf, x, promote.OutputJSON, true); err != nil {
		t.Fatalf("promote.Write() error = %v", err)
	}

	var got struct {
		Diagram struct {
			States map[string]any `json:"states"`
		} `json:"diagram"`
		Edges []struct {
			Origin string                   `json:"origin"`
			Guard  []promote.ExpandedClause `json:"guard"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v:\n%s", err, buf.String())
	}
	if len(got.Diagram.States) == 0 || len(got.Edges) != len(x.Edges) {
		t.Fatalf("WriteJSON() printed %d states and %d edges, want some states and %d edges", len(got.Diagram.States), len(got.Edges), len(x.Edges))
	}
	found := false
	for _, e := range got.Edges {
		for _, c := range e.Guard {
			found = found || c.Kind == promote.ClauseInDom
		}
	}
	if !found {
		t.Errorf("WriteJSON() printed no in_dom clause:\n%s", buf.String())
	}
}

// The PUML output is what String prints, with or without the origin comments.
func TestWritePUMLPrintsTheDiagram(t *testing.T) {
	x, _ := expand(t, syncGlobal, syncLocals)
	var with, without bytes.Buffer
	if err := promote.Write(&with, x, promote.OutputPUML, true); err != nil {
		t.Fatalf("promote.Write() error = %v", err)
	}
	if err := promote.Write(&without, x, promote.OutputPUML, false); err != nil {
		t.Fatalf("promote.Write() error = %v", err)
	}
	if with.String() != x.String() || without.String() != x.StringWithoutComments() {
		t.Error("promote.Write(OutputPUML) differs from String / StringWithoutComments")
	}
}
