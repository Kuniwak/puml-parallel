package csdfincludecmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v5/cli"
	"github.com/Kuniwak/puml-parallel/v5/csdf"
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/google/go-cmp/cmp"
)

// treeJSON is the JSON of an empty tree parsed from path.
func treeJSON(path string) string {
	p := ""
	if path != "" {
		p = `"path":"` + path + `",`
	}
	return `{"kind":"csdf-syntax-tree","version":1,` + p + `"name":"","line":1,"col":1,"body":[]}`
}

func emptyTree(path string) *csdf.SyntaxTree {
	return &csdf.SyntaxTree{Pos: csdf.Pos{Line: 1, Col: 1}, Path: path, Body: []csdf.Element{}}
}

func TestNewParseOptionsFuncOK(t *testing.T) {
	testCases := map[string]struct {
		Stdin    string
		Args     []string
		Expected *Options
	}{
		"-h": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"no args means stdin; the base is the directory of the tree's path": {
			Stdin:    treeJSON("path/to/G.puml"),
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Base: "path/to", Tree: emptyTree("path/to/G.puml")},
		},
		"dash means stdin; a tree with no path has the current directory as its base": {
			Stdin:    treeJSON(""),
			Args:     []string{"-"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Base: ".", Tree: emptyTree("")},
		},
		"-base overrides the derived base directory": {
			Stdin:    treeJSON("path/to/G.puml"),
			Args:     []string{"-base", "elsewhere"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Base: "elsewhere", Tree: emptyTree("path/to/G.puml")},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			opts, err := parseOptions(testCase.Args, spy.New())

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v: %s", err, spy.Stderr.String())
			}
			if !reflect.DeepEqual(testCase.Expected, opts) {
				t.Error(cmp.Diff(testCase.Expected, opts))
			}
		})
	}
}

func TestNewParseOptionsFuncNG(t *testing.T) {
	testCases := map[string]struct {
		Stdin string
		Args  []string
	}{
		"too many arguments":         {Args: []string{"a.json", "b.json"}},
		"PlantUML, naming csdfparse": {Stdin: "@startuml\n@enduml\n"},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			_, err := parseOptions(testCase.Args, spy.New())

			// Assert
			if err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}
