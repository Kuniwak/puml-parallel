package csdflivelockfreecmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/Kuniwak/puml-parallel/tools"
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
		"--help (representative value)": {
			Args:     []string{"--help"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"--version (representative value)": {
			Args:     []string{"--version"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"no args means stdin (representative value)": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: target.IRJSON,
				Bytes:  []byte("@startuml\n@enduml\n"),
			},
		},
		"dash means stdin (representative value)": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: target.IRJSON,
				Bytes:  []byte("@startuml\n@enduml\n"),
			},
		},
		"file argument (representative value)": {
			Args: []string{filepath.Join("testdata", "a.puml")},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: target.IRJSON,
				Bytes:  []byte("@startuml\n@enduml\n"),
			},
		},
		"-target isabelle": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-target", "isabelle"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: target.Isabelle,
				Bytes:  []byte("@startuml\n@enduml\n"),
			},
		},
		"-target lean": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-target", "lean"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: target.Lean,
				Bytes:  []byte("@startuml\n@enduml\n"),
			},
		},
		"-target ir-json": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-target", "ir-json"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: target.IRJSON,
				Bytes:  []byte("@startuml\n@enduml\n"),
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
		"too many arguments (representative value)": {
			Args: []string{"a.puml", "b.puml"},
		},
		"unknown target": {
			Args: []string{"-target", "bogus"},
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
