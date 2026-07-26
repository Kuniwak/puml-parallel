package csdfrefinementcmd

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
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

	spec := filepath.Join("testdata", "spec.puml")
	impl := filepath.Join("testdata", "impl.puml")
	specBytes := []byte("@startuml\nstate \"s0\" as s0\n[*] --> s0\ns0 --> s0 : a\n@enduml\n")
	implBytes := []byte("@startuml\nstate \"t0\" as t0\n[*] --> t0\nt0 --> t0 : a\n@enduml\n")

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
		"-m t (representative value)": {
			Args: []string{"-m", "t", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeTrace,
				Target: target.NameIRJSON,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"-m trace (representative value)": {
			Args: []string{"-m", "trace", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeTrace,
				Target: target.NameIRJSON,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"-m f (representative value)": {
			Args: []string{"-m", "f", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeStableFailure,
				Target: target.NameIRJSON,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"-m stable-failure (representative value)": {
			Args: []string{"-m", "stable-failure", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeStableFailure,
				Target: target.NameIRJSON,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"-m fd (representative value)": {
			Args: []string{"-m", "fd", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeFailuresDivergence,
				Target: target.NameIRJSON,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"-m failures-divergence (representative value)": {
			Args: []string{"-m", "failures-divergence", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeFailuresDivergence,
				Target: target.NameIRJSON,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"-target isabelle": {
			Args: []string{"-m", "t", "-target", "isabelle", spec, impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeTrace,
				Target: target.NameIsabelle,
				Spec:   specBytes,
				Impl:   implBytes,
			},
		},
		"dash means stdin (representative value)": {
			Stdin: "@startuml\n@enduml\n",
			Args:  []string{"-m", "t", "-", impl},
			Expected: &Options{
				Common: tools.NewCommonOptionsDefault(),
				Mode:   obligationir.IRRefinementModeTrace,
				Target: target.NameIRJSON,
				Spec:   []byte("@startuml\n@enduml\n"),
				Impl:   implBytes,
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
	spec := filepath.Join("testdata", "spec.puml")
	impl := filepath.Join("testdata", "impl.puml")

	testCases := map[string][]string{
		// -m has no default: which model a refinement is stated in changes what
		// the obligation means, so it cannot be guessed.
		"missing mode":           {spec, impl},
		"unknown mode":           {"-m", "bogus", spec, impl},
		"one file":               {"-m", "t", spec},
		"three files":            {"-m", "t", spec, impl, spec},
		"unknown target":         {"-m", "t", "-target", "bogus", spec, impl},
		"both files from stdin":  {"-m", "t", "-", "-"},
		"lean target (deferred)": {"-m", "t", "-target", "lean", spec, impl},
	}

	for name, args := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			parseOptions := NewParseOptionsFunc()
			spy := cli.SpyProcInout()

			// Act
			opts, err := parseOptions(args, spy.New())

			// Assert
			if err == nil {
				t.Log(opts)
				t.Fatal("want not nil, got nil")
			}
			if name == "lean target (deferred)" && !strings.Contains(err.Error(), "CSP-Prover") {
				t.Errorf("want the deferral reason in %q", err)
			}
		})
	}
}
