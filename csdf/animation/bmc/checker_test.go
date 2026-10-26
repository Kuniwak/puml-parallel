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

func TestCheckerArgsOfNextCarryTheEnvironment(t *testing.T) {
	glue := bmc.Checker{Checks: "c.py", EvaluatorModule: "g"}
	base := []string{"next", "d.json", "c.py", "--evaluator-module", "g"}
	tests := map[string]struct {
		env  []bmc.Binding
		want []string
	}{
		"nothing given": {want: base},
		"functions in the order given": {
			env:  []bmc.Binding{{Name: "$_0 が 正", Value: true}, {Name: "a=b", Value: false}},
			want: append(append([]string{}, base...), "--env", "$_0 が 正=true", "--env", "a=b=false"),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			checker := glue
			checker.Env = tt.env

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
	with := func(env ...bmc.Binding) bmc.Checker {
		c := glue
		c.Env = env
		return c
	}
	fTrue := bmc.Binding{Name: "f", Value: true}
	gave := func(env map[string]bool) *animation.Environment {
		given := []string{}
		for name := range env {
			given = append(given, name)
		}
		return &animation.Environment{Given: given, Env: env}
	}
	tests := map[string]struct {
		checker bmc.Checker
		report  *animation.Environment
		want    bmc.Checker
		wantErr string
	}{
		"nothing given on either side": {checker: glue, report: gave(nil), want: glue},
		"the report gives f, the checker nothing": {
			checker: glue, report: gave(map[string]bool{"f": true}), want: with(fTrue),
		},
		"the report and the checker give f alike": {
			checker: with(fTrue), report: gave(map[string]bool{"f": true}), want: with(fTrue),
		},
		"the report gives f another value": {
			checker: with(fTrue), report: gave(map[string]bool{"f": false}), wantErr: `the report's run gave "f" false, not true`,
		},
		"the checker gives f, the report nothing": {
			checker: with(fTrue), report: &animation.Environment{Given: []string{"f"}}, wantErr: `the checker gives "f" true, but the report's run gave it no value`,
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
	givesF := glue
	givesF.Env = []bmc.Binding{{Name: "f", Value: true}}
	tests := map[string]struct {
		checker bmc.Checker
		report  *animation.Environment
		wantErr string
	}{
		"nothing on either side":  {checker: glue, report: &animation.Environment{}},
		"the same function given": {checker: givesF, report: &animation.Environment{Given: []string{"f"}, Env: map[string]bool{"f": true}}},
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
