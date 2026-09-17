package csdftracechkcmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncOK(t *testing.T) {
	type testCase struct {
		Stdin    string
		Args     []string
		Expected *Options
	}

	diagram := "@startuml\nstate \"a\" as a\nstate \"b\" as b\n[*] --> a\na --> b : insert(coin) ; true ; coins' is {coin}\nb --> a : reset\n@enduml\n"
	okTrace := Trace{Name: filepath.Join("testdata", "ok.tsv"), Events: []csdf.Event{"insert(coin)", "reset"}}
	ngTrace := Trace{Name: filepath.Join("testdata", "ng.tsv"), Events: []csdf.Event{"reset"}}

	testCases := map[string]testCase{
		"-h (representative value)": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"diagram file and one trace": {
			Args: []string{filepath.Join("testdata", "a.puml"), okTrace.Name},
			Expected: &Options{
				Common:  tools.NewCommonOptionsDefault(),
				Match:   tracechk.MatchExact,
				Diagram: []byte(diagram),
				Traces:  []Trace{okTrace},
			},
		},
		"diagram from stdin and two traces": {
			Stdin: diagram,
			Args:  []string{"-", okTrace.Name, ngTrace.Name},
			Expected: &Options{
				Common:  tools.NewCommonOptionsDefault(),
				Match:   tracechk.MatchExact,
				Diagram: []byte(diagram),
				Traces:  []Trace{okTrace, ngTrace},
			},
		},
		"-match prefix": {
			Args: []string{"-match", "prefix", filepath.Join("testdata", "a.puml"), okTrace.Name},
			Expected: &Options{
				Common:  tools.NewCommonOptionsDefault(),
				Match:   tracechk.MatchPrefix,
				Diagram: []byte(diagram),
				Traces:  []Trace{okTrace},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Act
			opts, err := parseOptions(testCase.Args, spy.New())
			if err != nil {
				t.Log(spy.Stderr.String())
				t.Fatalf("want nil, got %#v", err)
			}

			// Assert
			if !reflect.DeepEqual(testCase.Expected, opts) {
				t.Error(cmp.Diff(testCase.Expected, opts))
			}
		})
	}
}

func TestNewParseOptionsFuncNG(t *testing.T) {
	testCases := map[string][]string{
		"no arguments":          {},
		"diagram without trace": {filepath.Join("testdata", "a.puml")},
		"unknown match rule":    {"-match", "bogus", filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "ok.tsv")},
		"missing trace file":    {filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "missing.tsv")},
		"malformed trace":       {filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "a.puml")},
		"trace from stdin":      {filepath.Join("testdata", "a.puml"), "-"},
	}

	for name, args := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(""))

			// Act
			opts, err := parseOptions(args, spy.New())

			// Assert
			if err == nil {
				t.Errorf("want an error, got %#v", opts)
			}
		})
	}
}
