package csdfslicecmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncOK(t *testing.T) {
	type testCase struct {
		Stdin    string
		Args     []string
		Expected *Options
	}

	testCases := map[string]testCase{
		"-h (representative value)": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"no -context means 1 (representative value)": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-states", "s0"},
			Expected: &Options{
				Common:  tools.NewCommonOptionsDefault(),
				States:  []string{"s0"},
				Context: 1,
				Bytes:   []byte("@startuml\n@enduml\n"),
			},
		},
		"-context 0 and two states (lower boundary value)": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-states", "s0,s1", "-context", "0", "-"},
			Expected: &Options{
				Common:  tools.NewCommonOptionsDefault(),
				States:  []string{"s0", "s1"},
				Context: 0,
				Bytes:   []byte("@startuml\n@enduml\n"),
			},
		},
		"file argument (representative value)": {
			Args: []string{"-states", "s0", filepath.Join("testdata", "a.puml")},
			Expected: &Options{
				Common:  tools.NewCommonOptionsDefault(),
				States:  []string{"s0"},
				Context: 1,
				Bytes:   []byte("@startuml\n@enduml\n"),
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
				t.Errorf("want nil, got %#v", err)
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
		"no -states (lower boundary value)": {
			Args: []string{filepath.Join("testdata", "a.puml")},
		},
		"an empty state in -states (representative value)": {
			Args: []string{"-states", "s0,", filepath.Join("testdata", "a.puml")},
		},
		"a negative -context (upper boundary value of the invalid)": {
			Args: []string{"-states", "s0", "-context", "-1", filepath.Join("testdata", "a.puml")},
		},
		"too many arguments (representative value)": {
			Args: []string{"-states", "s0", "a.puml", "b.puml"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()

			// Act
			opts, err := parseOptions(testCase.Args, spy.New())

			// Assert
			if err == nil {
				t.Log(opts)
				t.Error("want not nil, got nil")
			}
		})
	}
}
