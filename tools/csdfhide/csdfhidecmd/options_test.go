package csdfhidecmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncOK(t *testing.T) {
	type testCase struct {
		Args     []string
		Stdin    string
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
		"no events (lower boundary value)": {
			Args:     []string{"-"},
			Stdin:    "@startuml\n@enduml\n",
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Bytes: []byte("@startuml\n@enduml\n")},
		},
		"two events (representative value)": {
			Args:  []string{"-events", "x;y", "-"},
			Stdin: "@startuml\n@enduml\n",
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Events: []csdf.Event{"x", "y"},
				Bytes:  []byte("@startuml\n@enduml\n"),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader(testCase.Stdin)

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
		"too many arguments (representative value)": {
			Args: []string{"a.puml", "b.puml"},
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
