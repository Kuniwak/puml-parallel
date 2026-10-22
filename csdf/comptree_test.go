package csdf

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseExprReadsProcessExpressions(t *testing.T) {
	testCases := map[string]struct {
		Source   string
		Expected Expr
	}{
		"refer (the leaf of a composition tree)": {
			Source:   `{"op": "REFER", "path": "path/to/A.puml"}`,
			Expected: &ReferExpr{Path: "path/to/A.puml"},
		},
		"hide (wrapping a nested expression)": {
			Source:   `{"op": "HIDE", "channels": ["EVT_A"], "proc": {"op": "REFER", "path": "A.puml"}}`,
			Expected: &HideExpr{Channels: []string{"EVT_A"}, Proc: &ReferExpr{Path: "A.puml"}},
		},
		"interface parallel (holding a list of nested expressions)": {
			Source: `{"op": "INTERFACE_PARALLEL", "sync": ["EVT_A"], "procs": [{"op": "REFER", "path": "A.puml"}, {"op": "REFER", "path": "B.puml"}]}`,
			Expected: &InterfaceParallelExpr{
				Sync:  []string{"EVT_A"},
				Procs: []Expr{&ReferExpr{Path: "A.puml"}, &ReferExpr{Path: "B.puml"}},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseExpr([]byte(testCase.Source))
			if err != nil {
				t.Fatalf("want nil, got %#v", err)
			}

			if diff := cmp.Diff(testCase.Expected, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestParseExprRejectsInvalidTrees(t *testing.T) {
	testCases := map[string]string{
		"unknown op":                       `{"op": "SEQ", "procs": []}`,
		"refer without a path":             `{"op": "REFER"}`,
		"hide without a proc":              `{"op": "HIDE", "channels": ["a"]}`,
		"interface parallel without procs": `{"op": "INTERFACE_PARALLEL", "sync": []}`,
		"broken nested expression":         `{"op": "HIDE", "channels": ["a"], "proc": {"op": "REFER"}}`,
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

func TestBaseDirOf(t *testing.T) {
	testCases := map[string]struct {
		TreePath string
		Expected string
	}{
		"a tree file (representative value)":   {TreePath: "path/to/tree.json", Expected: "path/to"},
		"a tree file in the current directory": {TreePath: "tree.json", Expected: "."},
		"no location, e.g. standard input":     {TreePath: "", Expected: "."},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := BaseDirOf(testCase.TreePath); got != testCase.Expected {
				t.Errorf("want %q, got %q", testCase.Expected, got)
			}
		})
	}
}
