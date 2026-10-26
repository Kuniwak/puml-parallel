package csdfreplcmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
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
		"single file (representative value)": {
			Args:     []string{"a.puml"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), File: "a.puml"},
		},
		"a checks module and the glue csdfbmc reads the clauses through": {
			Args: []string{"-checks", "c.py", "-evaluator-module", "csdfbmc.python_glue", "a.puml"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), File: "a.puml",
				Checker: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue"}},
		},
		"the environment csdfbmc walks": {
			Args: []string{"-checks", "c.py", "-evaluator-module", "g", "-seed", "7", "-env", "f=false", "a.puml"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), File: "a.puml",
				Checker: bmc.Checker{Checks: "c.py", EvaluatorModule: "g", Seed: seed(7), Env: []bmc.Binding{{Name: "f", Value: false}}}},
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
		"missing argument (boundary value)": {
			Args: []string{},
		},
		"too many arguments (representative value)": {
			Args: []string{"a.puml", "b.puml"},
		},
		"a checks module without a glue": {
			Args: []string{"-checks", "c.py", "a.puml"},
		},
		"a seed without a checks module": {
			Args: []string{"-seed", "1", "a.puml"},
		},
		"a flag csdfrepl does not declare": {
			Args: []string{"-twins", "t.py", "a.puml"},
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

func TestNewParseOptionsFuncRefusesTheCheckerBeforeTheFiles(t *testing.T) {
	// Arrange
	parseOptions := NewParseOptionsFunc()
	spy := cli.SpyProcInout()

	// Act
	_, err := parseOptions([]string{"-checks", "c.py"}, spy.New())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "-checks needs") {
		t.Errorf("want the checker's error, as csdfrepld reports it, got %v", err)
	}
}

func seed(n int) *int { return &n }
