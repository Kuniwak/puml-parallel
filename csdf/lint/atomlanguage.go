package lint

import (
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
)

// AtomLinter is a glue asked whether an atom is of its language: the lint
// request of the evaluator protocol (csdfbmc/protocol.py). Lint returns why
// the atom is not one the glue reads in role, "guard" or "post", or "" when
// it is; an error is a glue that answered with one.
type AtomLinter interface {
	Lint(atom clause.AtomWire, role string) (string, error)
}

// AtomNotInLanguageRule reports an atom the glue does not read. It is the
// one rule that consults the language of the atoms, and only when a glue is
// given: a glue answers without values, so nothing is evaluated and no state
// is walked. Which atoms are environment functions is not written in the
// diagram (docs/CLAUSE_DESIGN.md), so every atom is asked; a diagram for
// csdfbmc is written in the glue's language alone. Each distinct atom is
// asked once per role. Findings are ERRORs.
type AtomNotInLanguageRule struct {
	Glue AtomLinter
}

func (AtomNotInLanguageRule) ID() string { return "atom-not-in-language" }

func (AtomNotInLanguageRule) Description() string {
	return "An atom the glue given by -evaluator does not read: not of its language, of another shape, or reading an undefined name."
}

func (r AtomNotInLanguageRule) Check(in *Input) []Finding {
	d := in.Diagram
	if d == nil {
		return nil
	}
	answers := map[string]string{}
	var findings []Finding
	check := func(f clause.And, role string, line int) {
		eachAtom(f, func(a clause.Atom) {
			if why := r.why(a, role, answers); why != "" {
				findings = append(findings, Finding{File: in.File, StartLine: line, EndLine: line, RuleID: r.ID(), Severity: SeverityError,
					Message: fmt.Sprintf("The %s atom `%s` is not one the glue reads: %s. Write it in the glue's language, or fix the glue's arguments if it reads a name they should define.", role, printedAtom(a), why)})
			}
		})
	}
	check(d.InitPost, "post", d.InitLine)
	for _, e := range d.Edges {
		check(e.Guard, "guard", e.Line)
		check(e.Post, "post", e.Line)
	}
	for _, e := range d.Ends {
		// An end edge keeps no line.
		check(e.Guard, "guard", 0)
	}
	return findings
}

// why asks the glue about a in role once, remembering the answer in answers.
func (r AtomNotInLanguageRule) why(a clause.Atom, role string, answers map[string]string) string {
	wire, _, err := clause.Wire(a)
	if err != nil {
		return err.Error()
	}
	b, err := json.Marshal(wire)
	if err != nil {
		return err.Error()
	}
	key := role + string(b)
	if why, ok := answers[key]; ok {
		return why
	}
	why, err := r.Glue.Lint(wire, role)
	if err != nil {
		why = "the glue answered " + err.Error()
	}
	answers[key] = why
	return why
}

// eachAtom calls visit on each atom of f, under any ∃.
func eachAtom(f clause.Formula, visit func(clause.Atom)) {
	switch f := f.(type) {
	case clause.And:
		for _, c := range f.Conjuncts {
			eachAtom(c, visit)
		}
	case clause.Exists:
		eachAtom(f.Body, visit)
	case clause.Atom:
		visit(f)
	}
}

func printedAtom(a clause.Atom) string {
	s, err := clause.Print(a)
	if err != nil {
		return fmt.Sprintf("%v", a)
	}
	return s
}
