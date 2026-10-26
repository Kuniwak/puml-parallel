package bmc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestCheckerValidate(t *testing.T) {
	tests := map[string]struct {
		checker bmc.Checker
		wantErr bool
	}{
		"none":                      {checker: bmc.Checker{}},
		"checks and a glue command": {checker: bmc.Checker{Checks: "c.py", Evaluator: "glue"}},
		"checks and a glue module":  {checker: bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue"}},
		"checks without a glue":     {checker: bmc.Checker{Checks: "c.py"}, wantErr: true},
		"a glue without checks":     {checker: bmc.Checker{EvaluatorModule: "m"}, wantErr: true},
		"both forms of the glue":    {checker: bmc.Checker{Checks: "c.py", Evaluator: "a", EvaluatorModule: "b"}, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := tt.checker.Validate()

			// Assert
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

// The message is csdfbmc's own for a walk without a glue
// (csdfbmc/tests/test_labels.py), so the REPLs and the checker say the same.
func TestCheckerWithoutAGlueSaysWhatCsdfbmcSays(t *testing.T) {
	// Execute
	err := bmc.Checker{Checks: "c.py"}.Validate()

	// Assert
	if err == nil || !strings.Contains(err.Error(), "csdfbmc reads every clause through one; pass -evaluator or -evaluator-module") {
		t.Errorf("Validate() = %v", err)
	}
}

func TestCheckerArgsOfNext(t *testing.T) {
	// Setup
	checker := bmc.Checker{Checks: "c.py", EvaluatorModule: "csdfbmc.python_glue defs.py"}

	// Execute
	got := checker.NextArgs("d.json")

	// Assert
	want := []string{"next", "d.json", "c.py", "--evaluator-module", "csdfbmc.python_glue defs.py"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NextArgs() mismatch (-want +got):\n%s", diff)
	}
}

func TestForSourceWritesTheDiagramAsCsdfparsePrintsIt(t *testing.T) {
	// Setup
	src := []byte("@startuml\nstate \"S\" as s0\ns0 : n\n[*] --> s0 : @eq(n', n)\n@enduml\n")

	// Execute
	s, cleanup, err := bmc.ForSource(src, bmc.Checker{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	raw, err := os.ReadFile(s.Diagram)
	if err != nil {
		t.Fatal(err)
	}
	d, err := core.UnmarshalDiagram(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Init != "s0" || len(d.Vars["s0"]) != 1 {
		t.Errorf("the diagram read back = %+v, want s0 holding n", d)
	}
}

func TestForSourceRefusesADiagramItCannotRead(t *testing.T) {
	// Execute
	_, _, err := bmc.ForSource([]byte("@startuml\n[*] --> \n@enduml\n"), bmc.Checker{})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "bmc.ForSource") {
		t.Errorf("ForSource error = %v, want one of ForSource", err)
	}
}

func seed(n int) *int { return &n }

func TestCheckerValidateRefusesAnEnvironmentWithoutChecks(t *testing.T) {
	tests := map[string]bmc.Checker{
		"a seed without checks":     {Seed: seed(1)},
		"a function without checks": {Env: []bmc.Binding{{Name: "$_0 が 正", Value: true}}},
	}
	for name, checker := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := checker.Validate()

			// Assert
			if err == nil || !strings.Contains(err.Error(), "need -checks") {
				t.Errorf("Validate() = %v, want -seed and -env to need -checks", err)
			}
		})
	}
}

func TestCheckerArgsOfNextCarryTheEnvironment(t *testing.T) {
	glue := bmc.Checker{Checks: "c.py", EvaluatorModule: "g"}
	base := []string{"next", "d.json", "c.py", "--evaluator-module", "g"}
	tests := map[string]struct {
		seed *int
		env  []bmc.Binding
		want []string
	}{
		"nothing given":  {want: base},
		"a seed of zero": {seed: seed(0), want: append(append([]string{}, base...), "--seed", "0")},
		"functions in the order given": {
			env:  []bmc.Binding{{Name: "$_0 が 正", Value: true}, {Name: "a=b", Value: false}},
			want: append(append([]string{}, base...), "--env", "$_0 が 正=true", "--env", "a=b=false"),
		},
		"a seed and a function": {
			seed: seed(7), env: []bmc.Binding{{Name: "f", Value: false}},
			want: append(append([]string{}, base...), "--seed", "7", "--env", "f=false"),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			checker := glue
			checker.Seed, checker.Env = tt.seed, tt.env

			// Execute
			got := checker.NextArgs("d.json")

			// Assert
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NextArgs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckerReproducesTheEnvironmentOfAReport(t *testing.T) {
	glue := bmc.Checker{Checks: "c.py", EvaluatorModule: "g"}
	with := func(s *int, env ...bmc.Binding) bmc.Checker {
		c := glue
		c.Seed, c.Env = s, env
		return c
	}
	f := bmc.Binding{Name: "f", Value: true}
	tests := map[string]struct {
		checker bmc.Checker
		report  *animation.Environment
		want    bmc.Checker
		wantErr string
	}{
		"a report of no environment": {checker: with(seed(1), f), report: nil, want: with(seed(1), f)},
		"the report's seed when none is given": {
			checker: glue, report: &animation.Environment{Seed: seed(7)}, want: with(seed(7)),
		},
		"no seed when the report had none": {
			checker: glue, report: &animation.Environment{}, want: glue,
		},
		"the same seed": {
			checker: with(seed(7)), report: &animation.Environment{Seed: seed(7)}, want: with(seed(7)),
		},
		"a function the report's run gave": {
			checker: with(nil, f), report: &animation.Environment{Seed: seed(7), Given: []string{"f"}}, want: with(seed(7), f),
		},
		"another seed": {
			checker: with(seed(1)), report: &animation.Environment{Seed: seed(7)}, wantErr: "the report was made with --seed 7, not -seed 1",
		},
		"a seed the report's run had not": {
			checker: with(seed(1)), report: &animation.Environment{}, wantErr: "the report was made with no seed, not -seed 1",
		},
		"a function the report's run did not give": {
			checker: with(nil, f), report: &animation.Environment{Seed: seed(7)}, wantErr: `-env gives "f", which the report's run did not give`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := tt.checker.Reproduce(tt.report)

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Reproduce() = %+v, %v; want an error saying %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Reproduce() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckerAgreesOnlyWithTheEnvironmentItAlreadyWalks(t *testing.T) {
	glue := bmc.Checker{Checks: "c.py", EvaluatorModule: "g"}
	seeded := glue
	seeded.Seed = seed(7)
	tests := map[string]struct {
		checker bmc.Checker
		report  *animation.Environment
		wantErr string
	}{
		"a report of no environment": {checker: glue, report: nil},
		"the same seed":              {checker: seeded, report: &animation.Environment{Seed: seed(7)}},
		"no seed on either side":     {checker: glue, report: &animation.Environment{}},
		"a seed it was not given":    {checker: glue, report: &animation.Environment{Seed: seed(7)}, wantErr: "the report was made with --seed 7; give -seed 7"},
		"another seed":               {checker: seeded, report: &animation.Environment{Seed: seed(1)}, wantErr: "not -seed 7"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			err := tt.checker.Agrees(tt.report)

			// Assert
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Agrees() = %v, want an error saying %q", err, tt.wantErr)
			}
		})
	}
}
