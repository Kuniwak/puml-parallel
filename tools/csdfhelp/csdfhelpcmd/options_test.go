package csdfhelpcmd

import (
	"reflect"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncOK(t *testing.T) {
	type testCase struct {
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
		"no arguments selects every tool (lower boundary value)": {
			Args:     []string{},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), ToolNames: []string{}},
		},
		"-short (representative value)": {
			Args:     []string{"-short"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Short: true, ToolNames: []string{}},
		},
		"single tool name (lower boundary value)": {
			Args:     []string{"csdfparse"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), ToolNames: []string{"csdfparse"}},
		},
		"two tool names (representative value)": {
			Args:     []string{"csdfparse", "csdfevents"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), ToolNames: []string{"csdfparse", "csdfevents"}},
		},
		"csdfhelp itself is a known tool (representative value)": {
			Args:     []string{"csdfhelp"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), ToolNames: []string{"csdfhelp"}},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()

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
		"unknown tool (representative value)": {
			Args: []string{"csdfrowidx"},
		},
		"unknown tool among known ones (representative value)": {
			Args: []string{"csdfparse", "csdfrowidx"},
		},
		"undefined flag (representative value)": {
			Args: []string{"-unknown"},
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
