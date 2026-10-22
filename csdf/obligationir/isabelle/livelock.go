package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
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

// WriteLivelockFree writes the livelock-freedom obligation of a
// diagram whose clauses are formulas. The clauses are spelled, not left
// opaque: what stays uninterpreted is the environment, each atom, function
// and type a constant under a TODO(csdf) marker beside the text it was named
// by, and the theorem is stated under the axioms of the csdf_* predicates and
// the diagram's assumptions.
func WriteLivelockFree(w io.Writer, ir obligationir.LivelockFree) error {
	text, err := livelockFree(ir)
	if err != nil {
		return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
	}
	_, err = io.WriteString(w, text)
	return err
}

// livelockFree is the theory WriteLivelockFree writes, made
// whole before any of it is written, so that an IR refused halfway writes
// nothing.
func livelockFree(ir obligationir.LivelockFree) (string, error) {
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
	ds, err := ir.Declarations(PredicateTODO)
	if err != nil {
		return "", err
	}
	writeStates(&w, side, ir.States)
	writeSymbols(&w, ds)
	hypotheses, err := writeAxioms(&w, ir.Axioms)
	if err != nil {
		return "", err
	}
	if err := WriteAssumptions(&w, side, ir.Assumptions); err != nil {
		return "", err
	}
	hypotheses = append(hypotheses, AssumptionNames(side, ir.Assumptions)...)
	if err := writeTransitionSystem(&w, side, ir); err != nil {
		return "", err
	}
	WriteLivelockTheorem(&w, side, "livelock_free", hypotheses)
	io.WriteString(&w, "\n\nend\n")
	return w.String(), nil
}

// writeTransitionSystem writes the relations the livelock-freedom theorem of
// one side is stated over: init, step, reachable and tau_step, each named
// for the side, over the states the datatype st of the side holds, which
// writeStates writes.
func writeTransitionSystem(w io.Writer, side obligationir.Side, ir obligationir.LivelockFree) error {
	st, init, reachable := side.Qualify("st"), side.Qualify("init"), side.Qualify("reachable")
	states := ir.StateMap()
	initial, err := obligationir.InitDisjunct(Notation, side, states[ir.Init], ir.InitPost, constructor)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "definition %s :: \"%s \\<Rightarrow> bool\"\n  where \"%s s' \\<equiv> %s\"\n\n", init, st, init, initial)
	if err := writeRelation(w, side, "step", states, ir.Edges); err != nil {
		return err
	}
	fmt.Fprintf(w, `inductive %s :: "%s \<Rightarrow> bool"
  where base: "%s s \<Longrightarrow> %s s"
  | step: "%s s \<Longrightarrow> %s s s' \<Longrightarrow> %s s'"

`, reachable, st, init, reachable, reachable, side.Qualify("step"), reachable)
	return writeRelation(w, side, "tau_step", states, ir.TauEdges())
}

func constructor(ctor string) string { return ctor }

func writeStates(w io.Writer, side obligationir.Side, ss []obligationir.State) {
	io.WriteString(w, "datatype "+side.Qualify("st")+" =")
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

func writeSymbols(w io.Writer, ds []obligationir.Declaration) {
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
}

func writeAxioms(w io.Writer, axioms []logic.Tree) ([]string, error) {
	names := make([]string, len(axioms))
	for i, a := range axioms {
		names[i] = obligationir.AxiomName(i + 1)
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
	st, name := side.Qualify("st"), side.Qualify(name)
	fmt.Fprintf(w, "definition %s :: \"%s \\<Rightarrow> %s \\<Rightarrow> bool\"\n  where \"%s s s' \\<equiv> %s\"\n\n", name, st, st, name, d)
	return nil
}

// comment is s as an Isabelle comment, on one line and never closed early.
func comment(s string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }), " ")
	return "(* " + strings.ReplaceAll(s, "*)", "* )") + " *)"
}
