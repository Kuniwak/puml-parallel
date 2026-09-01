package csdfcompcmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncOK(t *testing.T) {
	type testCase struct {
		Args     []string
		Stdin    string
		Expected *Options
	}

	tree := `{"op": "REFER", "path": "a.puml"}`

	testCases := map[string]testCase{
		"-h (representative value)": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"stdin (representative value)": {
			Args:     []string{"-"},
			Stdin:    tree,
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), BaseDir: ".", Bytes: []byte(tree)},
		},
		"-base overrides the derived base directory (representative value)": {
			Args:     []string{"-base", "elsewhere", "-"},
			Stdin:    tree,
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), BaseDir: "elsewhere", Bytes: []byte(tree)},
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

func TestNewParseOptionsFuncDerivesBaseDirFromTheTreeFile(t *testing.T) {
	// Arrange
	parseOptions := NewParseOptionsFunc()
	spy := cli.SpyProcInout()

	// Act
	opts, err := parseOptions([]string{"../../../examples/valid/in_out_tree.json"}, spy.New())
	if err != nil {
		t.Log(spy.Stderr.String())
		t.Fatalf("want nil, got %#v", err)
	}

	// Assert
	if want := "../../../examples/valid"; opts.BaseDir != want {
		t.Errorf("want %q, got %q", want, opts.BaseDir)
	}
}

func TestNewParseOptionsFuncNG(t *testing.T) {
	type testCase struct {
		Args []string
	}

	testCases := map[string]testCase{
		"too many arguments (representative value)": {
			Args: []string{"a.json", "b.json"},
		},
		"missing file (representative value)": {
			Args: []string{"missing.json"},
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
