package csdf

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseExprReadsReferExpr(t *testing.T) {
	// Setup: the leaf of a composition tree is a path to an existing diagram.
	got, err := ParseExpr([]byte(`{"op": "REFER", "path": "path/to/A.puml"}`))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	want := Expr(&ReferExpr{Path: "path/to/A.puml"})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestParseExprReadsHideExpr(t *testing.T) {
	// Setup: a hiding wraps a nested process expression.
	got, err := ParseExpr([]byte(`{"op": "HIDE", "events": ["EVT-A"], "proc": {"op": "REFER", "path": "A.puml"}}`))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	want := Expr(&HideExpr{Events: []Event{"EVT-A"}, Proc: &ReferExpr{Path: "A.puml"}})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestParseExprReadsInterfaceParallelExpr(t *testing.T) {
	// Setup: an interface parallel holds a list of nested process expressions.
	got, err := ParseExpr([]byte(`{"op": "INTERFACE_PARALLEL", "sync": ["EVT-A"], "procs": [{"op": "REFER", "path": "A.puml"}, {"op": "REFER", "path": "B.puml"}]}`))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	want := Expr(&InterfaceParallelExpr{
		Sync:  []Event{"EVT-A"},
		Procs: []Expr{&ReferExpr{Path: "A.puml"}, &ReferExpr{Path: "B.puml"}},
	})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestParseExprRejectsInvalidTrees(t *testing.T) {
	testCases := map[string]string{
		"unknown op":                       `{"op": "SEQ", "procs": []}`,
		"refer without a path":             `{"op": "REFER"}`,
		"hide without a proc":              `{"op": "HIDE", "events": ["a"]}`,
		"interface parallel without procs": `{"op": "INTERFACE_PARALLEL", "sync": []}`,
		"broken nested expression":         `{"op": "HIDE", "events": ["a"], "proc": {"op": "REFER"}}`,
		"not an object":                    `["op"]`,
	}

	for name, source := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseExpr([]byte(source))
			if err == nil {
				t.Errorf("want not nil, got nil: %#v", got)
			}
		})
	}
}

func TestComposeTreeComposesAndHidesReferencedDiagrams(t *testing.T) {
	// Setup: the tree of the two example diagrams synchronised on "sync",
	// with "sync" hidden afterwards.
	expr, err := ParseExpr([]byte(`{
		"op": "HIDE",
		"proc": {
			"op": "INTERFACE_PARALLEL",
			"sync": ["sync"],
			"procs": [
				{"op": "REFER", "path": "in.puml"},
				{"op": "REFER", "path": "out.puml"}
			]
		},
		"events": ["sync"]
	}`))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	got, err := ComposeTree(expr, NewFileDiagramLoader("../examples/valid"))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}

	want := `@startuml
state "(s0, s0)" as s0_s0
state "(s1, s0)" as s1_s0
state "(s2, s1)" as s2_s1
state "(s2, s2)" as s2_s2
[*] --> s0_s0
s0_s0 --> s1_s0 : in
s1_s0 --> s2_s1 : tau
s2_s1 --> s2_s2 : out
@enduml
`
	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Error(diff)
	}
}

func TestComposeTreeReportsUnreadableReferences(t *testing.T) {
	// Setup: a REFER pointing at a missing file must name the path it failed on.
	expr := Expr(&ReferExpr{Path: "missing.puml"})

	_, err := ComposeTree(expr, NewFileDiagramLoader("../examples/valid"))

	if err == nil {
		t.Fatal("want not nil, got nil")
	}
	if !strings.Contains(err.Error(), "missing.puml") {
		t.Errorf("want an error naming missing.puml, got %q", err)
	}
}
