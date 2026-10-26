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
	"sort"
	"strconv"

	"github.com/Kuniwak/puml-parallel/csdf/animation"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
)

// Checker is what csdfbmc reads besides the diagram: the checks module of the
// domains, ghosts and invariants, and the glue every clause is read through,
// as a command line (Evaluator) or a Python module and its arguments
// (EvaluatorModule); and the environment, as csdfbmc takes it: Env (--env, in the
// order given). The zero Checker asks nothing.
type Checker struct {
	Checks          string
	Evaluator       string
	EvaluatorModule string
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

// Asks reports whether the Checker is set, so that value prompts suggest.
func (c Checker) Asks() bool { return c.Checks != "" }

// NextArgs are the arguments of "csdfbmc next" for the diagram JSON at path,
// before what next is asked.
func (c Checker) NextArgs(diagram string) []string {
	args := []string{"next", diagram, c.Checks}
	if c.Evaluator != "" {
		args = append(args, "--evaluator", c.Evaluator)
	} else {
		args = append(args, "--evaluator-module", c.EvaluatorModule)
	}
	for _, b := range c.Env {
		args = append(args, "--env", b.String())
	}
	return args
}

// Suggester runs csdfbmc: diagram is the JSON the checker reads (csdfparse's
// output, or an expansion from csdfpromote -format json), checker what it
// reads besides, and run what runs csdfbmc.
type Suggester struct {
	diagram string
	checker Checker
	run     Runner
}

// NewSuggester is the Suggester asking, through run (RunCsdfbmc, or a
// stand-in), csdfbmc next about the diagram JSON at path diagram.
func NewSuggester(diagram string, checker Checker, run Runner) *Suggester {
	return &Suggester{diagram: diagram, checker: checker, run: run}
}

// Diagram is the path of the diagram JSON the suggester asks about.
func (s *Suggester) Diagram() string { return s.diagram }

// Runner runs csdfbmc with args and returns what it printed on its standard
// output, or an error saying why it failed.
type Runner func(args []string) ([]byte, error)

// Suggesters is the factory of the suggesters c and run make for a
// session: one reading the diagram JSON when the session explores one,
// otherwise one reading the source as csdfparse prints it (ForSource).
func (c Checker) Suggesters(run Runner) animation.SuggesterFactory {
	return func(source []byte, diagramJSON string) (animation.Suggester, func(), error) {
		if diagramJSON != "" {
			return NewSuggester(diagramJSON, c, run), animation.NoCleanup, nil
		}
		return ForSource(source, c, run)
	}
}

// ForSource writes the diagram whose PlantUML source (or PNG) is src as
// csdfparse prints it to a temporary file, and returns a Suggester reading
// it, so a REPL started on a .puml needs no JSON of its own. The caller
// removes the file with the returned function.
func ForSource(src []byte, checker Checker, run Runner) (*Suggester, func(), error) {
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
	return NewSuggester(f.Name(), checker, run), func() { _ = os.Remove(f.Name()) }, nil
}

func (s *Suggester) Initial() ([]animation.Suggestion, error) {
	return s.ask("--initial")
}

func (s *Suggester) Next(from animation.State, idx int) ([]animation.Suggestion, error) {
	state, err := json.Marshal(from)
	if err != nil {
		return nil, fmt.Errorf("bmc.Suggester.Next: %w", err)
	}
	return s.ask("--state", string(state), "--edge", strconv.Itoa(idx))
}

func (s *Suggester) ask(args ...string) ([]animation.Suggestion, error) {
	stdout, err := s.run(append(s.checker.NextArgs(s.diagram), args...))
	if err != nil {
		return nil, err
	}
	var out struct {
		Candidates []animation.Suggestion `json:"candidates"`
	}
	if err := json.Unmarshal(stdout, &out); err != nil {
		return nil, fmt.Errorf("bmc.Suggester: csdfbmc next printed something that is not its JSON: %w", err)
	}
	if out.Candidates == nil {
		out.Candidates = []animation.Suggestion{}
	}
	return out.Candidates, nil
}

// RunCsdfbmc runs the csdfbmc on the PATH, or beside the running binary (a
// release unpacks every tool together), failing with what it printed on its
// standard error.
func RunCsdfbmc(args []string) ([]byte, error) {
	command, err := executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(command, args...)
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
	return stdout.Bytes(), nil
}

func executable() (string, error) {
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

// errNoEnvironment refuses a report from a csdfbmc that did not yet report
// its environment: what its run walked cannot be told.
var errNoEnvironment = errors.New("the report records no environment, so the run that made it cannot be reproduced; make it again with this release's csdfbmc --json")

// Reproduce is c walking the environment of the csdfbmc run that made a
// report (animation.LoadEnvironment), so that a replayed finding is
// explored further in the environment it was found in: the values its run
// gave the functions, which c takes where it says nothing. A value other
// than the report's,
// or a function c gives a value the report's run did not give one (its
// checks module did, or nothing did) is refused, for c would walk another
// environment.
func (c Checker) Reproduce(report *animation.Environment) (Checker, error) {
	if report == nil {
		return Checker{}, errNoEnvironment
	}
	for _, b := range c.Env {
		v, ok := report.Env[b.Name]
		if !ok {
			return Checker{}, fmt.Errorf("the checker gives %q %t, but the report's run gave it no value", b.Name, b.Value)
		}
		if v != b.Value {
			return Checker{}, fmt.Errorf("the report's run gave %q %t, not %t", b.Name, v, b.Value)
		}
	}
	out := c
	out.Env = bindings(report.Env)
	return out, nil
}

// bindings are the values env gives, ordered by name.
func bindings(env map[string]bool) []Binding {
	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []Binding
	for _, name := range names {
		out = append(out, Binding{Name: name, Value: env[name]})
	}
	return out
}

// Agrees refuses a report whose environment c does not already walk: what
// Reproduce refuses, and a value c does not give. A
// daemon's checker is fixed when it starts, so it cannot take the report's
// environment as csdfrepl does.
func (c Checker) Agrees(report *animation.Environment) error {
	if _, err := c.Reproduce(report); err != nil {
		return err
	}
	given := map[string]bool{}
	for _, b := range c.Env {
		given[b.Name] = true
	}
	for _, b := range bindings(report.Env) {
		if !given[b.Name] {
			return fmt.Errorf("the report's run gave %q %t, which the checker does not give", b.Name, b.Value)
		}
	}
	return nil
}

// LoadFinding reads csdfbmc's JSON output and returns its n-th finding
// (animation.LoadFinding) and the checker that explores it further: c
// walking the environment the report was made in (Reproduce), or c as it
// is when it asks nothing.
func LoadFinding(bs []byte, n int, c Checker) (animation.Finding, Checker, error) {
	finding, env, err := animation.LoadFinding(bs, n)
	if err != nil {
		return animation.Finding{}, Checker{}, fmt.Errorf("bmc.LoadFinding: %w", err)
	}
	if !c.Asks() {
		return finding, c, nil
	}
	reproduced, err := c.Reproduce(env)
	if err != nil {
		return animation.Finding{}, Checker{}, fmt.Errorf("bmc.LoadFinding: %w", err)
	}
	return finding, reproduced, nil
}
