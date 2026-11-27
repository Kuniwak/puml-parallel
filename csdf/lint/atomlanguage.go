package lint

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v8/csdf/clause"
)

// Role is the clause an atom is in, as the lint request names it.
type Role string

const (
	RoleGuard Role = "guard"
	RolePost  Role = "post"
)

// AtomLint asks a glue whether an atom is of its language: the lint request
// of the evaluator protocol (csdfbmc/protocol.py). It returns why the glue
// does not read atom in role, or "" when it does; an error is a glue that
// could not answer, or answered with one.
type AtomLint func(atom clause.AtomWire, role Role) (string, error)

// AtomNotInLanguageRule reports an atom the glue does not read. It is the
// one rule that consults the language of the atoms, and only when a glue is
// given: a glue answers without values, so nothing is evaluated and no state
// is walked. Which atoms are environment functions is not written in the
// diagram (docs/CLAUSE_DESIGN.md), so every atom is asked; a diagram for
// csdfbmc is written in the glue's language alone. Each distinct atom is
// asked once per role. Findings are ERRORs. A glue that fails says nothing
// of the atoms: its error goes to Fail, not into a finding, and nothing more
// is asked of it.
type AtomNotInLanguageRule struct {
	Lint AtomLint
	Fail func(error)
}

func (AtomNotInLanguageRule) ID() string { return "atom-not-in-language" }

func (AtomNotInLanguageRule) Description() string {
	return "An atom the glue given by -evaluator or -evaluator-module does not read: not of its language, of another shape, or reading an undefined name."
}

func (r AtomNotInLanguageRule) Check(in *Input) []Finding {
	d := in.Diagram
	if d == nil {
		return nil
	}
	answers := map[string]string{}
	failed := false
	var findings []Finding
	check := func(f clause.And, role Role, line int) {
		clause.EachAtom(f, func(a clause.Atom) {
			if failed {
				return
			}
			why, err := r.why(a, role, answers)
			if err != nil {
				failed = true
				r.Fail(err)
				return
			}
			if why != "" {
				findings = append(findings, Finding{File: in.File, StartLine: line, EndLine: line, RuleID: r.ID(), Severity: SeverityError,
					Message: fmt.Sprintf("The %s atom `%s` is not one the glue reads: %s. Write it in the glue's language, or fix the glue's arguments if it reads a name they should define.", role, printedAtom(a), why)})
			}
		})
	}
	check(d.InitPost, RolePost, d.InitLine)
	for _, e := range d.Edges {
		check(e.Guard, RoleGuard, e.Line)
		check(e.Post, RolePost, e.Line)
	}
	for _, e := range d.Ends {
		// An end edge keeps no line.
		check(e.Guard, RoleGuard, 0)
	}
	return findings
}

// why asks the glue about a in role once, remembering the answer in answers.
// An atom the glue cannot be sent is not one it reads; an error is the
// glue's.
func (r AtomNotInLanguageRule) why(a clause.Atom, role Role, answers map[string]string) (string, error) {
	wire, _, err := clause.Wire(a)
	if err != nil {
		return err.Error(), nil
	}
	b, err := json.Marshal(wire)
	if err != nil {
		return err.Error(), nil
	}
	key := string(role) + string(b)
	if why, ok := answers[key]; ok {
		return why, nil
	}
	why, err := r.Lint(wire, role)
	if err != nil {
		return "", err
	}
	answers[key] = why
	return why, nil
}

func printedAtom(a clause.Atom) string {
	s, err := clause.Print(a)
	if err != nil {
		return fmt.Sprintf("%v", a)
	}
	return s
}
