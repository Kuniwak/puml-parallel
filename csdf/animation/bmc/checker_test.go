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
	fTrue := bmc.Binding{Name: "f", Value: true}
	gave := func(s *int, env map[string]bool) *animation.Environment {
		given := []string{}
		for name := range env {
			given = append(given, name)
		}
		return &animation.Environment{Seed: s, Given: given, Env: env}
	}
	tests := map[string]struct {
		checker bmc.Checker
		report  *animation.Environment
		want    bmc.Checker
		wantErr string
	}{
		"the report's seed when none is given": {checker: glue, report: gave(seed(7), nil), want: with(seed(7))},
		"no seed when the report had none":     {checker: glue, report: gave(nil, nil), want: glue},
		"the same seed":                        {checker: with(seed(7)), report: gave(seed(7), nil), want: with(seed(7))},
		"the report gives f, the checker nothing": {
			checker: glue, report: gave(seed(7), map[string]bool{"f": true}), want: with(seed(7), fTrue),
		},
		"the report and the checker give f alike": {
			checker: with(nil, fTrue), report: gave(seed(7), map[string]bool{"f": true}), want: with(seed(7), fTrue),
		},
		"the report gives f another value": {
			checker: with(nil, fTrue), report: gave(nil, map[string]bool{"f": false}), wantErr: `the report's run gave "f" false, not true`,
		},
		"the checker gives f, the report nothing": {
			checker: with(nil, fTrue), report: &animation.Environment{Given: []string{"f"}}, wantErr: `the checker gives "f" true, but the report's run gave it no value`,
		},
		"another seed": {
			checker: with(seed(1)), report: gave(seed(7), nil), wantErr: "the report was made with seed 7, not seed 1",
		},
		"a seed the report's run had not": {
			checker: with(seed(1)), report: gave(nil, nil), wantErr: "the report was made with no seed, not seed 1",
		},
		"a report of no environment": {checker: glue, report: nil, wantErr: "the report records no environment"},
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
	givesF := glue
	givesF.Env = []bmc.Binding{{Name: "f", Value: true}}
	tests := map[string]struct {
		checker bmc.Checker
		report  *animation.Environment
		wantErr string
	}{
		"the same seed":           {checker: seeded, report: &animation.Environment{Seed: seed(7)}},
		"no seed on either side":  {checker: glue, report: &animation.Environment{}},
		"the same function given": {checker: givesF, report: &animation.Environment{Given: []string{"f"}, Env: map[string]bool{"f": true}}},
		"a seed it was not given": {checker: glue, report: &animation.Environment{Seed: seed(7)}, wantErr: "the report was made with seed 7, which the checker does not walk"},
		"another seed":            {checker: seeded, report: &animation.Environment{Seed: seed(1)}, wantErr: "not seed 7"},
		"a function it does not give": {
			checker: glue, report: &animation.Environment{Given: []string{"f"}, Env: map[string]bool{"f": true}}, wantErr: `the report's run gave "f" true, which the checker does not give`,
		},
		"a report of no environment": {checker: glue, report: nil, wantErr: "the report records no environment"},
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
