package bmc_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
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

// command writes a csdfbmc that runs "next" from the checkout.
func (c checker) command(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "csdfbmc")
	script := "#!/bin/sh\n[ \"$1\" = next ] || exit 9\nshift\nPYTHONPATH=" + c.root + " exec " + c.python + " -s -B -m csdfbmc.next \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
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

var zero = csdf.RuntimeState{ID: "s0", Values: []csdf.StateValue{{Name: "n", Value: 0}}}

func TestSuggesterWithASeedSuggestsWhatCsdfbmcNextDoes(t *testing.T) {
	c := newChecker(t)
	for _, n := range []int{0, 1, 2, 3} {
		t.Run("seed "+strconv.Itoa(n), func(t *testing.T) {
			// Setup
			s, cleanup, err := bmc.ForSource([]byte(guarded), bmc.Checker{Checks: c.checks, EvaluatorModule: "csdfbmc.python_glue", Seed: seed(n)})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			s.Command = c.command(t)
			state, _ := json.Marshal(zero)
			want := c.next(t, s.Diagram, "--seed", strconv.Itoa(n), "--state", string(state), "--edge", "0")

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

func TestSuggesterTakesTheFunctionsGivenOverTheSeed(t *testing.T) {
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
			s, cleanup, err := bmc.ForSource([]byte(guarded), bmc.Checker{Checks: c.checks, EvaluatorModule: "csdfbmc.python_glue", Seed: seed(1), Env: tt.env})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			s.Command = c.command(t)

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
			checker: bmc.Checker{Seed: seed(1), Env: []bmc.Binding{{Name: "$_0 が 零", Value: true}}},
			want:    "ENVIRONMENT does not declare",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			checker := tt.checker
			checker.Checks, checker.EvaluatorModule = c.checks, "csdfbmc.python_glue"
			s, cleanup, err := bmc.ForSource([]byte(guarded), checker)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			s.Command = c.command(t)

			// Execute
			_, err = s.Initial()

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Initial() = %v, want an error saying %q", err, tt.want)
			}
		})
	}
}
