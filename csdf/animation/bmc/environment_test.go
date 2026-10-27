package bmc_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
	"github.com/google/go-cmp/cmp"
)

// guarded is a diagram whose one edge is guarded by an atom no glue reads,
// which the checks module declares a function of the environment.
const guarded = "@startuml\nstate \"S\" as s0\ns0 : n\n[*] --> s0 : @eq(n', 0)\ns0 --> s0 : inc?x ; $x が 正 ; @eq(n', x)\n@enduml\n"

const declared = "TYPES = {\"n\": [0, 1, 2, 3], \"x\": [0, 1, 2, 3]}\nENVIRONMENT = {\"$_0 が 正\": None}\n"

// checker runs the checker of this checkout as the csdfbmc command would,
// so that a test sees what the Python says and not a copy of it.
type checker struct {
	python, root, checks string
}

func newChecker(t *testing.T) checker {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not on the PATH")
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	checks := filepath.Join(t.TempDir(), "checks.py")
	if err := os.WriteFile(checks, []byte(declared), 0o644); err != nil {
		t.Fatal(err)
	}
	return checker{python: python, root: root, checks: checks}
}

// run runs "csdfbmc next" from the checkout.
func (c checker) run(t *testing.T) bmc.Runner {
	t.Helper()
	return func(args []string) ([]byte, error) {
		if len(args) == 0 || args[0] != "next" {
			t.Fatalf("csdfbmc %v, want next", args)
		}
		cmd := exec.Command(c.python, append([]string{"-s", "-B", "-m", "csdfbmc.next"}, args[1:]...)...)
		cmd.Env = append(os.Environ(), "PYTHONPATH="+c.root)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, errors.New(stderr.String())
		}
		return out, nil
	}
}

// next asks csdfbmc next directly, with the given arguments after the inputs.
func (c checker) next(t *testing.T, diagram string, args ...string) []animation.Suggestion {
	t.Helper()
	full := append([]string{"-s", "-B", "-m", "csdfbmc.next", diagram, c.checks, "--evaluator-module", "csdfbmc.python_glue"}, args...)
	cmd := exec.Command(c.python, full...)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+c.root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("csdfbmc next %v: %v", args, err)
	}
	var got struct {
		Candidates []animation.Suggestion `json:"candidates"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	return got.Candidates
}

var zero = animation.State{ID: "s0", Values: []animation.Value{{Name: "n", Value: 0}}}

func TestSuggesterGivenAFunctionSuggestsWhatCsdfbmcNextDoes(t *testing.T) {
	c := newChecker(t)
	for _, v := range []string{"true", "false"} {
		t.Run(v, func(t *testing.T) {
			// Setup
			env := []bmc.Binding{{Name: "$_0 が 正", Value: v == "true"}}
			s, cleanup, err := bmc.ForSource([]byte(guarded), bmc.Checker{Checks: c.checks, EvaluatorModule: "csdfbmc.python_glue", Env: env}, c.run(t))
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			state, _ := json.Marshal(zero)
			want := c.next(t, s.Diagram(), "--env", env[0].String(), "--state", string(state), "--edge", "0")

			// Execute
			got, err := s.Next(zero, 0)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Next() mismatch (-csdfbmc next +suggester):\n%s", diff)
			}
		})
	}
}

func TestSuggesterTakesTheFunctionsGiven(t *testing.T) {
	c := newChecker(t)
	tests := map[string]struct {
		env       []bmc.Binding
		wantCount int
	}{
		"given false": {env: []bmc.Binding{{Name: "$_0 が 正", Value: false}}, wantCount: 0},
		"given true":  {env: []bmc.Binding{{Name: "$_0 が 正", Value: true}}, wantCount: 4},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			s, cleanup, err := bmc.ForSource([]byte(guarded), bmc.Checker{Checks: c.checks, EvaluatorModule: "csdfbmc.python_glue", Env: tt.env}, c.run(t))
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			// Execute
			got, err := s.Next(zero, 0)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("Next() = %v, want %d candidates", got, tt.wantCount)
			}
		})
	}
}

func TestSuggesterRefusesWhatCsdfbmcRefuses(t *testing.T) {
	c := newChecker(t)
	tests := map[string]struct {
		checker bmc.Checker
		want    string
	}{
		"a function nothing decides": {
			checker: bmc.Checker{},
			want:    "nothing decides the environment's functions '$_0 が 正'",
		},
		"a function the checks module does not declare": {
			checker: bmc.Checker{Env: []bmc.Binding{{Name: "$_0 が 零", Value: true}}},
			want:    "ENVIRONMENT does not declare",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			checker := tt.checker
			checker.Checks, checker.EvaluatorModule = c.checks, "csdfbmc.python_glue"
			s, cleanup, err := bmc.ForSource([]byte(guarded), checker, c.run(t))
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			// Execute
			_, err = s.Initial()

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Initial() = %v, want an error saying %q", err, tt.want)
			}
		})
	}
}

// parallel has two edges of one event between the same states, told apart
// only by their posts; the index a session selects must be the one csdfbmc
// next numbers the edge by.
const parallel = "@startuml\nstate \"S\" as s0\ns0 : n\n[*] --> s0 : $n' == 0\ns0 --> s0 : tick ; ; $n' == 1\ns0 --> s0 : tick ; ; $n' == 2\n@enduml\n"

func TestASelectedParallelEdgeIsTheOneCsdfbmcNumbersSo(t *testing.T) {
	c := newChecker(t)
	if err := os.WriteFile(c.checks, []byte("TYPES = {\"n\": [0, 1, 2]}\n"), 0o644); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	tests := map[string]struct {
		idx  int
		want string
	}{
		"the first of the parallel edges":  {idx: 0, want: "[1]"},
		"the second of the parallel edges": {idx: 1, want: "[2]"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			checker := bmc.Checker{Checks: c.checks, EvaluatorModule: "csdfbmc.python_glue"}
			s, cleanup, err := bmc.ForSource([]byte(parallel), checker, c.run(t))
			if err != nil {
				t.Fatalf("Setup: ForSource() error = %v", err)
			}
			defer cleanup()
			d, _, err := animation.ReadDiagram([]byte(parallel), nil, nil)
			if err != nil {
				t.Fatalf("Setup: ReadDiagram() error = %v", err)
			}
			session, err := animation.NewSession(d, s)
			if err != nil {
				t.Fatalf("Setup: NewSession() error = %v", err)
			}
			if _, err := session.EnterValues("[0]"); err != nil {
				t.Fatalf("Setup: EnterValues() error = %v", err)
			}
			if err := session.Select(tt.idx); err != nil {
				t.Fatalf("Setup: Select() error = %v", err)
			}

			// Execute
			got, err := session.Suggest()

			// Assert
			if err != nil {
				t.Fatalf("Suggest() error = %v", err)
			}
			if len(got) != 1 || string(got[0].Values) != tt.want {
				t.Errorf("Suggest() = %v, want only %s", got, tt.want)
			}
		})
	}
}

func TestASessionTakesOnlyTheValuesCsdfbmcAdmits(t *testing.T) {
	c := newChecker(t)
	if err := os.WriteFile(c.checks, []byte("TYPES = {\"n\": [0, 1, 2]}\n"), 0o644); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	tests := map[string]struct {
		values  string
		wantErr string
	}{
		"the value the selected edge's post admits": {values: "[2]"},
		"the value the other parallel edge's post admits": {
			values:  "[1]",
			wantErr: "the checker admits none of these values; enter one of its suggestions",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			checker := bmc.Checker{Checks: c.checks, EvaluatorModule: "csdfbmc.python_glue"}
			s, cleanup, err := bmc.ForSource([]byte(parallel), checker, c.run(t))
			if err != nil {
				t.Fatalf("Setup: ForSource() error = %v", err)
			}
			defer cleanup()
			d, _, err := animation.ReadDiagram([]byte(parallel), nil, nil)
			if err != nil {
				t.Fatalf("Setup: ReadDiagram() error = %v", err)
			}
			session, err := animation.NewSession(d, s)
			if err != nil {
				t.Fatalf("Setup: NewSession() error = %v", err)
			}
			if _, err := session.EnterValues("[0]"); err != nil {
				t.Fatalf("Setup: EnterValues() error = %v", err)
			}
			if err := session.Select(1); err != nil {
				t.Fatalf("Setup: Select() error = %v", err)
			}

			// Execute
			_, err = session.EnterValues(tt.values)

			// Assert
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("EnterValues() error = %v, want the values taken", err)
				}
				return
			}
			var refused *animation.RefusedValuesError
			if !errors.As(err, &refused) || refused.Error() != tt.wantErr {
				t.Fatalf("EnterValues() error = %v, want refused values saying %q", err, tt.wantErr)
			}
		})
	}
}
