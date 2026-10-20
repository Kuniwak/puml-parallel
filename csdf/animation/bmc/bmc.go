// Package bmc is the animation.Suggester backed by the csdfbmc checker: it
// asks "csdfbmc next" what values the twins admit at a value prompt.
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

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/animation"
)

// Suggester runs csdfbmc. Diagram is the JSON the checker reads (csdfparse's
// output, or an expansion from csdfpromote -format json), Twins the twins
// module, Locals the per-family twins of an expansion as PATH=TWINS.py, and
// Command the csdfbmc executable (found on the PATH, or beside this binary,
// when empty).
type Suggester struct {
	Diagram string
	Twins   string
	Locals  []string
	Command string
}

// ForDiagram writes d as csdfparse would print it to a temporary file and
// returns a Suggester reading it, so a REPL started on a .puml needs no JSON
// of its own. The caller removes the file with the returned function.
func ForDiagram(d *csdf.Diagram, twins string, locals []string) (*Suggester, func(), error) {
	f, err := os.CreateTemp("", "csdfrepl-*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("bmc.ForDiagram: %w", err)
	}
	if err := json.NewEncoder(f).Encode(d); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, nil, fmt.Errorf("bmc.ForDiagram: %w", err)
	}
	_ = f.Close()
	return &Suggester{Diagram: f.Name(), Twins: twins, Locals: locals}, func() { _ = os.Remove(f.Name()) }, nil
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
	full := []string{"next", s.Diagram, s.Twins}
	for _, l := range s.Locals {
		full = append(full, "--local", l)
	}
	full = append(full, args...)
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
