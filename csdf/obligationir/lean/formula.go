package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// Notation spells the formulas of an obligation in Lean 4, every value of
// the type Val.
var Notation = obligationir.Notation{
	And: ` ∧ `, Or: ` ∨ `, Not: `¬`, Implies: ` → `, True: `True`, False: `False`,
	Exists: func(vars []string, body string) string { return `∃ ` + binders(vars) + `, ` + body },
	Forall: func(vars []string, body string) string { return `∀ ` + binders(vars) + `, ` + body },
}

func binders(vars []string) string {
	bs := make([]string, len(vars))
	for i, v := range vars {
		bs[i] = `(` + v + ` : Val)`
	}
	return strings.Join(bs, ` `)
}

// InhabitedVal lets a function into Val be declared opaque, which Lean
// allows only of an inhabited type.
const InhabitedVal = `instance : Inhabited Val := ⟨.ValBool false⟩`

// WriteFormulaLivelockFree writes the livelock-freedom obligation of a
// diagram whose clauses are formulas, as the Isabelle back end does
// (isabelle.WriteFormulaLivelockFree).
func WriteFormulaLivelockFree(w io.Writer, ir obligationir.LivelockFree) error {
	if ir.Structurally {
		io.WriteString(w, "-- Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation.\n")
		return nil
	}
	side := obligationir.SideSingle
	io.WriteString(w, ValPrelude+"\n\n"+InhabitedVal+"\n\n")
	writeStates(w, side, ir.States)
	writeSymbols(w, ir)
	hypotheses := writeAxioms(w, ir)
	if err := WriteAssumptions(w, side, ir.Assumptions); err != nil {
		return fmt.Errorf("lean.WriteFormulaLivelockFree: %w", err)
	}
	hypotheses = append(hypotheses, AssumptionNames(side, ir.Assumptions)...)

	states := ir.StateMap()
	fmt.Fprintf(w, "def init (s' : St) : Prop :=\n  %s\n\n", obligationir.InitDisjunct(Notation, side, states[ir.Init], ir.InitPost, dotted))
	writeRelation(w, side, "step", states, ir.Edges)
	io.WriteString(w, `inductive Reachable : St → Prop where
  | base (s : St) : init s → Reachable s
  | step (s s' : St) : Reachable s → step s s' → Reachable s'

`)
	writeRelation(w, side, "tauStep", states, ir.TauEdges())
	WriteLivelockTheorem(w, side, "livelock_free", hypotheses)
	io.WriteString(w, "\n")
	return nil
}

func dotted(ctor string) string { return "." + ctor }

func writeStates(w io.Writer, side obligationir.Side, ss []obligationir.State) {
	io.WriteString(w, "inductive St where\n")
	for _, s := range ss {
		io.WriteString(w, "  | "+side.Ctor(csdf.StateID(s.Name)))
		for _, v := range s.Vars {
			io.WriteString(w, " ("+obligationir.VarName(v)+" : Val)")
		}
		if obligationir.IsMangled(s.Name) || len(s.Vars) > 0 {
			io.WriteString(w, " -- "+SanitizeComment(s.Name+obligationir.VarsComment(s.Vars)))
		}
		io.WriteString(w, "\n")
	}
	io.WriteString(w, "\n")
}

func writeSymbols(w io.Writer, ir obligationir.LivelockFree) {
	for _, d := range ir.Declarations(PredicateTODO) {
		for _, c := range d.Comments {
			io.WriteString(w, "-- "+SanitizeComment(c)+"\n")
		}
		result := "Val"
		if d.Pred {
			result = "Prop"
		}
		io.WriteString(w, "opaque "+d.Name+" : "+strings.Repeat("Val → ", d.Arity)+result+"\n\n")
	}
}

func writeAxioms(w io.Writer, ir obligationir.LivelockFree) []string {
	names := ir.AxiomNames()
	for i, a := range ir.Axioms {
		fmt.Fprintf(w, "def %s : Prop :=\n  %s\n\n", names[i], Notation.SpellFormula(a))
	}
	return names
}

func writeRelation(w io.Writer, side obligationir.Side, name string, states map[string]obligationir.State, edges []obligationir.Edge) {
	fmt.Fprintf(w, "def %s (s s' : St) : Prop :=\n  %s\n\n", name, obligationir.StepDisjuncts(Notation, side, states, edges, dotted))
}
