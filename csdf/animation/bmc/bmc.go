// Package bmc is the animation.Suggester backed by the csdfbmc checker: it
// asks "csdfbmc next" what values the clauses admit at a value prompt.
package bmc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
)

// Checker is what csdfbmc reads besides the diagram: the checks module of the
// domains, ghosts and invariants, and the glue every clause is read through,
// as a command line (Evaluator) or a Python module and its arguments
// (EvaluatorModule); and the environment, as csdfbmc takes it: Seed (--seed,
// nil when none is given) and Env (--env, in the order given). The zero
// Checker asks nothing.
type Checker struct {
	Checks          string
	Evaluator       string
	EvaluatorModule string
	Seed            *int
	Env             []Binding
}

// Binding gives a function of the environment, named as csdfbmc names it
// (the atom's text with its holes as $_0, $_1, ...), a truth value: what
// "--env NAME=true|false" says.
type Binding struct {
	Name  string
	Value bool
}

// String is the binding as --env takes it.
func (b Binding) String() string { return b.Name + "=" + strconv.FormatBool(b.Value) }

// ParseBinding reads NAME=true or NAME=false as csdfbmc reads --env: the
// name is everything before the last "=", and must not be empty.
func ParseBinding(word string) (Binding, error) {
	i := strings.LastIndex(word, "=")
	if i <= 0 || (word[i+1:] != "true" && word[i+1:] != "false") {
		return Binding{}, fmt.Errorf("-env expects NAME=true or NAME=false, not %q", word)
	}
	return Binding{Name: word[:i], Value: word[i+1:] == "true"}, nil
}

// Asks reports whether the Checker is set, so that value prompts suggest.
func (c Checker) Asks() bool { return c.Checks != "" }

// Validate refuses a Checker that csdfbmc could not run: a checks module
// needs a glue, a glue needs a checks module, and the glue has one form.
func (c Checker) Validate() error {
	glue := c.Evaluator != "" || c.EvaluatorModule != ""
	switch {
	case c.Evaluator != "" && c.EvaluatorModule != "":
		return errors.New("give -evaluator or -evaluator-module, not both")
	case c.Checks != "" && !glue:
		return errors.New("-checks needs a glue: csdfbmc reads every clause through one; pass -evaluator or -evaluator-module (csdfbmc.python_glue reads Python)")
	case glue && c.Checks == "":
		return errors.New("-evaluator and -evaluator-module need -checks, the module of the domains")
	case (c.Seed != nil || len(c.Env) > 0) && c.Checks == "":
		return errors.New("-seed and -env need -checks, whose ENVIRONMENT declares the functions they decide")
	}
	return nil
}

// NextArgs are the arguments of "csdfbmc next" for the diagram JSON at path,
// before what next is asked.
func (c Checker) NextArgs(diagram string) []string {
	args := []string{"next", diagram, c.Checks}
	if c.Evaluator != "" {
		args = append(args, "--evaluator", c.Evaluator)
	} else {
		args = append(args, "--evaluator-module", c.EvaluatorModule)
	}
	if c.Seed != nil {
		args = append(args, "--seed", strconv.Itoa(*c.Seed))
	}
	for _, b := range c.Env {
		args = append(args, "--env", b.String())
	}
	return args
}

// Suggester runs csdfbmc. Diagram is the JSON the checker reads (csdfparse's
// output, or an expansion from csdfpromote -format json), Checker what it
// reads besides, and Command the csdfbmc executable (found on the PATH, or
// beside this binary, when empty).
type Suggester struct {
	Diagram string
	Checker Checker
	Command string
}

// ForSource writes the diagram whose PlantUML source (or PNG) is src as
// csdfparse prints it to a temporary file, and returns a Suggester reading
// it, so a REPL started on a .puml needs no JSON of its own. The caller
// removes the file with the returned function.
func ForSource(src []byte, checker Checker) (*Suggester, func(), error) {
	d, err := coreio.ReadBytes(src)
	if err != nil {
		return nil, nil, fmt.Errorf("bmc.ForSource: %w", err)
	}
	raw, err := core.MarshalDiagram(d)
	if err != nil {
		return nil, nil, fmt.Errorf("bmc.ForSource: %w", err)
	}
	f, err := os.CreateTemp("", "csdfrepl-*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("bmc.ForSource: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, nil, fmt.Errorf("bmc.ForSource: %w", err)
	}
	_ = f.Close()
	return &Suggester{Diagram: f.Name(), Checker: checker}, func() { _ = os.Remove(f.Name()) }, nil
}

func (s *Suggester) Initial() ([]animation.Suggestion, error) {
	return s.run("--initial")
}

func (s *Suggester) Next(from csdf.RuntimeState, idx int) ([]animation.Suggestion, error) {
	state, err := json.Marshal(from)
	if err != nil {
		return nil, fmt.Errorf("bmc.Suggester.Next: %w", err)
	}
	return s.run("--state", string(state), "--edge", strconv.Itoa(idx))
}

func (s *Suggester) run(args ...string) ([]animation.Suggestion, error) {
	command, err := s.executable()
	if err != nil {
		return nil, err
	}
	full := append(s.Checker.NextArgs(s.Diagram), args...)
	cmd := exec.Command(command, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := bytes.TrimSpace(stderr.Bytes())
		if len(msg) == 0 {
			return nil, fmt.Errorf("bmc.Suggester: %s failed: %w", command, err)
		}
		return nil, fmt.Errorf("bmc.Suggester: %s", msg)
	}
	var out struct {
		Candidates []animation.Suggestion `json:"candidates"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("bmc.Suggester: csdfbmc next printed something that is not its JSON: %w", err)
	}
	if out.Candidates == nil {
		out.Candidates = []animation.Suggestion{}
	}
	return out.Candidates, nil
}

// executable finds csdfbmc: the configured command, the PATH, or the
// directory of the running binary (a release unpacks every tool together).
func (s *Suggester) executable() (string, error) {
	if s.Command != "" {
		return s.Command, nil
	}
	if path, err := exec.LookPath("csdfbmc"); err == nil {
		return path, nil
	}
	if self, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(self), "csdfbmc")
		if _, err := os.Stat(beside); err == nil {
			return beside, nil
		}
	}
	return "", errors.New("bmc.Suggester: csdfbmc is not on the PATH nor beside this binary")
}
