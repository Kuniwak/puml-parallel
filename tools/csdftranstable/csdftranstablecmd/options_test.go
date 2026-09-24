package csdftranstablecmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
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

	testCases := map[string]testCase{
		"-h (representative value)": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"a file argument, with the defaults": {
			Args: []string{filepath.Join("testdata", "a.puml")},
			Expected: &Options{
				Common:   tools.NewCommonOptionsDefault(),
				ExprMode: ExprModeNatural,
				Bytes:    []byte(diagram),
			},
		},
		"standard input (equivalent to a file argument)": {
			Stdin: diagram,
			Args:  []string{},
			Expected: &Options{
				Common:   tools.NewCommonOptionsDefault(),
				ExprMode: ExprModeNatural,
				Bytes:    []byte(diagram),
			},
		},
		"-post and -expr-mode logical": {
			Args: []string{"-post", "-expr-mode", "logical", filepath.Join("testdata", "a.puml")},
			Expected: &Options{
				Common:   tools.NewCommonOptionsDefault(),
				Post:     true,
				ExprMode: ExprModeLogical,
				Bytes:    []byte(diagram),
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
		"unknown expression mode": {"-expr-mode", "bogus", filepath.Join("testdata", "a.puml")},
		"too many arguments":      {filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "a.puml")},
		"missing file":            {filepath.Join("testdata", "missing.puml")},
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
