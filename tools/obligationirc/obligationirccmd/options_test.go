package obligationirccmd

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
		"no args means stdin, default target": {
			Stdin: "{}\n",
			Args:  []string{},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: TargetIRJSON,
				Bytes:  []byte("{}\n"),
			},
		},
		"dash means stdin": {
			Stdin: "{}\n",
			Args:  []string{"-"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: TargetIRJSON,
				Bytes:  []byte("{}\n"),
			},
		},
		"file argument": {
			Args: []string{filepath.Join("testdata", "ir.json")},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: TargetIRJSON,
				Bytes:  []byte("{\"goal\":\"livelock_free\"}\n"),
			},
		},
		"-target isabelle": {
			Stdin: "{}\n",
			Args:  []string{"-target", "isabelle"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: TargetIsabelle,
				Bytes:  []byte("{}\n"),
			},
		},
		"-target lean": {
			Stdin: "{}\n",
			Args:  []string{"-target", "lean"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: TargetLean,
				Bytes:  []byte("{}\n"),
			},
		},
		"-target ir-json": {
			Stdin: "{}\n",
			Args:  []string{"-target", "ir-json"},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Target: TargetIRJSON,
				Bytes:  []byte("{}\n"),
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
		"too many arguments": {
			Args: []string{"a.json", "b.json"},
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
