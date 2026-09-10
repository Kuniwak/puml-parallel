package csdflintcmd

import (
	"os"
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

	source := "@startuml\n@enduml\n"

	testCases := map[string]testCase{
		"-h (representative value)": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"no args means stdin (lower boundary value)": {
			Stdin: source,
			Args:  []string{},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Inputs: []tools.FileInput{{Name: "-", Bytes: []byte(source)}},
			},
		},
		"two file arguments (representative value)": {
			Args: []string{filepath.Join("testdata", "clean.puml"), filepath.Join("testdata", "selfloop.puml")},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Inputs: []tools.FileInput{
					{Name: filepath.Join("testdata", "clean.puml"), Bytes: mustRead(t, "clean.puml")},
					{Name: filepath.Join("testdata", "selfloop.puml"), Bytes: mustRead(t, "selfloop.puml")},
				},
			},
		},
		// The catalog is about the tool and not about any input, so asking for
		// it must not block on a terminal that will never be typed into.
		"-list-rules reads no input (representative value)": {
			Args:     []string{"-list-rules"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), ListRules: true},
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
		Args []string
	}

	testCases := map[string]testCase{
		"an unknown flag (representative value)": {
			Args: []string{"-nope"},
		},
		"a file that does not exist (representative value)": {
			Args: []string{filepath.Join("testdata", "missing.puml")},
		},
		`two "-" arguments (representative value)`: {
			Args: []string{"-", "-"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader("@startuml\n@enduml\n"))

			// Act
			opts, err := parseOptions(testCase.Args, spy.New())

			// Assert
			if err == nil {
				t.Errorf("want an error, got %#v", opts)
			}
		})
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()

	bs, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("want nil, got %#v", err)
	}
	return bs
}
