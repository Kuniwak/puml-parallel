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
	io.WriteString(w, "theory Livelock_Obligation\n  imports Main\nbegin\n\n")
	if ir.Structurally {
		io.WriteString(w, "(* Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation. *)\n\nend\n")
		return nil
	}
	side := obligationir.SideSingle
	io.WriteString(w, ValPrelude+"\n\n")
	writeStates(w, side, ir.States)
	writeSymbols(w, ir)
	hypotheses := writeAxioms(w, ir)
	if err := WriteAssumptions(w, side, ir.Assumptions); err != nil {
		return fmt.Errorf("isabelle.WriteFormulaLivelockFree: %w", err)
	}
	hypotheses = append(hypotheses, AssumptionNames(side, ir.Assumptions)...)

	states := map[string]obligationir.State{}
	for _, s := range ir.States {
		states[s.Name] = s
	}
	init := obligationir.InitDisjunct(Notation, side, states[ir.Init], ir.InitPost, func(ctor string) string { return ctor })
	fmt.Fprintf(w, "definition init :: \"st \\<Rightarrow> bool\"\n  where \"init s' \\<equiv> %s\"\n\n", init)
	writeRelation(w, side, "step", states, ir.Edges)
	io.WriteString(w, `inductive reachable :: "st \<Rightarrow> bool"
  where base: "init s \<Longrightarrow> reachable s"
  | step: "reachable s \<Longrightarrow> step s s' \<Longrightarrow> reachable s'"

`)
	var taus []obligationir.Edge
	for _, e := range ir.Edges {
		if e.Tau {
			taus = append(taus, e)
		}
	}
	writeRelation(w, side, "tau_step", states, taus)
	WriteLivelockTheorem(w, side, "livelock_free", hypotheses)
	io.WriteString(w, "\n\nend\n")
	return nil
}

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

func writeSymbols(w io.Writer, ir obligationir.LivelockFree) {
	for _, s := range ir.Symbols() {
		if obligationir.Uninterpreted(s) {
			io.WriteString(w, comment(PredicateTODO)+"\n")
		}
		if s.Text != "" {
			io.WriteString(w, comment(s.Text)+"\n")
		}
		result := "val"
		if s.Pred {
			result = "bool"
		}
		io.WriteString(w, "consts "+s.Name+" :: \""+strings.Repeat("val \\<Rightarrow> ", s.Arity)+result+"\"\n\n")
	}
}

func writeAxioms(w io.Writer, ir obligationir.LivelockFree) []string {
	names := make([]string, len(ir.Axioms))
	for i, a := range ir.Axioms {
		names[i] = obligationir.AxiomName(i + 1)
		fmt.Fprintf(w, "definition %s :: bool\n  where \"%s \\<equiv> %s\"\n\n", names[i], names[i], Notation.SpellFormula(a))
	}
	return names
}

func writeRelation(w io.Writer, side obligationir.Side, name string, states map[string]obligationir.State, edges []obligationir.Edge) {
	fmt.Fprintf(w, "definition %s :: \"st \\<Rightarrow> st \\<Rightarrow> bool\"\n  where \"%s s s' \\<equiv> %s\"\n\n",
		name, name, obligationir.StepDisjuncts(Notation, side, states, edges, func(ctor string) string { return ctor }))
}

// comment is s as an Isabelle comment, on one line and never closed early.
func comment(s string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }), " ")
	return "(* " + strings.ReplaceAll(s, "*)", "* )") + " *)"
}
