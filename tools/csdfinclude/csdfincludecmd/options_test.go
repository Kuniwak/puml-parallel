package csdfincludecmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v5/cli"
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/google/go-cmp/cmp"
)

func TestNewParseOptionsFuncOK(t *testing.T) {
	testCases := map[string]struct {
		Stdin    string
		Args     []string
		Expected *Options
	}{
		"-h": {
			Args:     []string{"-h"},
			Expected: &Options{Common: tools.CommonOptionsHelp},
		},
		"-v": {
			Args:     []string{"-v"},
			Expected: &Options{Common: tools.CommonOptionsVersion},
		},
		"no args means stdin": {
			Stdin:    "{}",
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Bytes: []byte("{}")},
		},
		"dash means stdin": {
			Stdin:    "{}",
			Args:     []string{"-"},
			Expected: &Options{Common: tools.NewCommonOptionsDefault(), Bytes: []byte("{}")},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(testCase.Stdin))

			// Execute
			opts, err := parseOptions(testCase.Args, spy.New())

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v: %s", err, spy.Stderr.String())
			}
			if !reflect.DeepEqual(testCase.Expected, opts) {
				t.Error(cmp.Diff(testCase.Expected, opts))
			}
		})
	}
}

func TestNewParseOptionsFuncRefusesTooManyArguments(t *testing.T) {
	// Setup
	parseOptions := NewParseOptionsFunc()
	spy := cli.SpyProcInout()

	// Execute
	_, err := parseOptions([]string{"a.json", "b.json"}, spy.New())

	// Assert
	if err == nil {
		t.Error("want an error, got nil")
	}
}
