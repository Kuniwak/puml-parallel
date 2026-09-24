package csdftranstablecmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/transtable"
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
				ExprMode: transtable.NotationNatural,
				Bytes:    []byte(diagram),
			},
		},
		"standard input (equivalent to a file argument)": {
			Stdin: diagram,
			Args:  []string{},
			Expected: &Options{
				Common:   tools.NewCommonOptionsDefault(),
				ExprMode: transtable.NotationNatural,
				Bytes:    []byte(diagram),
			},
		},
		"-expr-mode logical": {
			Args: []string{"-expr-mode", "logical", filepath.Join("testdata", "a.puml")},
			Expected: &Options{
				Common:   tools.NewCommonOptionsDefault(),
				ExprMode: transtable.NotationLogical,
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
	type testCase struct {
		Args        []string
		WantInError string
	}

	testCases := map[string]testCase{
		"unknown expression mode": {
			Args:        []string{"-expr-mode", "bogus", filepath.Join("testdata", "a.puml")},
			WantInError: `unknown notation "bogus"`,
		},
		"-post, which is gone": {
			Args:        []string{"-post", filepath.Join("testdata", "a.puml")},
			WantInError: "flag provided but not defined: -post",
		},
		"too many arguments": {
			Args:        []string{filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "a.puml")},
			WantInError: "too many arguments",
		},
		"missing file": {
			Args:        []string{filepath.Join("testdata", "missing.puml")},
			WantInError: "cannot read file",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(""))

			// Act
			opts, err := parseOptions(testCase.Args, spy.New())

			// Assert
			if err == nil {
				t.Fatalf("want an error, got %#v", opts)
			}
			if !strings.Contains(err.Error(), testCase.WantInError) {
				t.Errorf("want %q in the error, got %q", testCase.WantInError, err.Error())
			}
		})
	}
}
