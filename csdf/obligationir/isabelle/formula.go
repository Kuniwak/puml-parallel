package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// Notation spells the formulas of an obligation in Isabelle/HOL, every value
// of the type val.
var Notation = obligationir.Notation{
	And: ` \<and> `, Or: ` \<or> `, Not: `\<not> `, Implies: ` \<longrightarrow> `, True: `True`, False: `False`,
	Exists: func(vars []string, body string) string { return `\<exists>` + binders(vars) + `. ` + body },
	Forall: func(vars []string, body string) string { return `\<forall>` + binders(vars) + `. ` + body },
}

func binders(vars []string) string {
	bs := make([]string, len(vars))
	for i, v := range vars {
		bs[i] = `(` + v + `::val)`
	}
	return strings.Join(bs, ` `)
}

// WriteFormulaLivelockFree writes the livelock-freedom obligation of a
// diagram whose clauses are formulas. The clauses are spelled, not left
// opaque: what stays uninterpreted is the environment, each atom, function
// and type a constant under a TODO(csdf) marker beside the text it was named
// by, and the theorem is stated under the axioms of the csdf_* predicates and
// the diagram's assumptions.
func WriteFormulaLivelockFree(w io.Writer, ir obligationir.LivelockFree) error {
	text, err := formulaLivelockFree(ir)
	if err != nil {
		return fmt.Errorf("isabelle.WriteFormulaLivelockFree: %w", err)
	}
	_, err = io.WriteString(w, text)
	return err
}

// formulaLivelockFree is the theory WriteFormulaLivelockFree writes, made
// whole before any of it is written, so that an IR refused halfway writes
// nothing.
func formulaLivelockFree(ir obligationir.LivelockFree) (string, error) {
	if err := ir.Validate(); err != nil {
		return "", err
	}
	var w strings.Builder
	io.WriteString(&w, "theory Livelock_Obligation\n  imports Main\nbegin\n\n")
	if ir.Structurally {
		io.WriteString(&w, "(* Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation. *)\n\nend\n")
		return w.String(), nil
	}
	side := obligationir.SideSingle
	io.WriteString(&w, ValPrelude+"\n\n")
	writeStates(&w, side, ir.States)
	if err := writeSymbols(&w, ir); err != nil {
		return "", err
	}
	hypotheses, err := writeAxioms(&w, ir)
	if err != nil {
		return "", err
	}
	if err := WriteAssumptions(&w, side, ir.Assumptions); err != nil {
		return "", err
	}
	hypotheses = append(hypotheses, AssumptionNames(side, ir.Assumptions)...)

	states := ir.StateMap()
	init, err := obligationir.InitDisjunct(Notation, side, states[ir.Init], ir.InitPost, constructor)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&w, "definition init :: \"st \\<Rightarrow> bool\"\n  where \"init s' \\<equiv> %s\"\n\n", init)
	if err := writeRelation(&w, side, "step", states, ir.Edges); err != nil {
		return "", err
	}
	io.WriteString(&w, `inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

`)
	if err := writeRelation(&w, side, "tau_step", states, ir.TauEdges()); err != nil {
		return "", err
	}
	WriteLivelockTheorem(&w, side, "livelock_free", hypotheses)
	io.WriteString(&w, "\n\nend\n")
	return w.String(), nil
}

func constructor(ctor string) string { return ctor }

func writeStates(w io.Writer, side obligationir.Side, ss []obligationir.State) {
	io.WriteString(w, "datatype st =")
	for i, s := range ss {
		if i > 0 {
			io.WriteString(w, "\n  |")
		}
		io.WriteString(w, " "+side.Ctor(csdf.StateID(s.Name)))
		for range s.Vars {
			io.WriteString(w, " val")
		}
		if obligationir.IsMangled(s.Name) || len(s.Vars) > 0 {
			io.WriteString(w, " "+comment(s.Name+obligationir.VarsComment(s.Vars)))
		}
	}
	io.WriteString(w, "\n\n")
}

func writeSymbols(w io.Writer, ir obligationir.LivelockFree) error {
	ds, err := ir.Declarations(PredicateTODO)
	if err != nil {
		return err
	}
	for _, d := range ds {
		for _, c := range d.Comments {
			io.WriteString(w, comment(c)+"\n")
		}
		result := "val"
		if d.Pred {
			result = "bool"
		}
		io.WriteString(w, "consts "+d.Name+" :: \""+strings.Repeat("val \\<Rightarrow> ", d.Arity)+result+"\"\n\n")
	}
	return nil
}

func writeAxioms(w io.Writer, ir obligationir.LivelockFree) ([]string, error) {
	names := ir.AxiomNames()
	for i, a := range ir.Axioms {
		f, err := Notation.SpellFormula(a)
		if err != nil {
			return nil, fmt.Errorf("the axiom %s: %w", names[i], err)
		}
		fmt.Fprintf(w, "definition %s :: bool\n  where \"%s \\<equiv> %s\"\n\n", names[i], names[i], f)
	}
	return names, nil
}

func writeRelation(w io.Writer, side obligationir.Side, name string, states map[string]obligationir.State, edges []obligationir.Edge) error {
	d, err := obligationir.StepDisjuncts(Notation, side, states, edges, constructor)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "definition %s :: \"st \\<Rightarrow> st \\<Rightarrow> bool\"\n  where \"%s s s' \\<equiv> %s\"\n\n", name, name, d)
	return nil
}

// comment is s as an Isabelle comment, on one line and never closed early.
func comment(s string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }), " ")
	return "(* " + strings.ReplaceAll(s, "*)", "* )") + " *)"
}
