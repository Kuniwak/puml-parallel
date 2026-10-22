package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
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
	text, err := formulaLivelockFree(ir)
	if err != nil {
		return fmt.Errorf("lean.WriteFormulaLivelockFree: %w", err)
	}
	_, err = io.WriteString(w, text)
	return err
}

// formulaLivelockFree is the file WriteFormulaLivelockFree writes, made whole
// before any of it is written, so that an IR refused halfway writes nothing.
func formulaLivelockFree(ir obligationir.LivelockFree) (string, error) {
	if err := ir.Validate(); err != nil {
		return "", err
	}
	var w strings.Builder
	if ir.Structurally {
		return "-- Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation.\n", nil
	}
	side := obligationir.SideSingle
	io.WriteString(&w, ValPrelude+"\n\n"+InhabitedVal+"\n\n")
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
	io.WriteString(&w, "\n")
	return w.String(), nil
}

// writeTransitionSystem writes the relations the livelock-freedom theorem of
// one side is stated over: init, step, Reachable and tauStep, each named for
// the side, over the states the inductive St of the side holds, which
// writeStates writes.
func writeTransitionSystem(w io.Writer, side obligationir.Side, ir obligationir.LivelockFree) error {
	st, init, reachable := side.Qualify("St"), side.Qualify("init"), side.Qualify("Reachable")
	states := ir.StateMap()
	initial, err := obligationir.InitDisjunct(Notation, side, states[ir.Init], ir.InitPost, dotted)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "def %s (s' : %s) : Prop :=\n  %s\n\n", init, st, initial)
	if err := writeRelation(w, side, "step", states, ir.Edges); err != nil {
		return err
	}
	fmt.Fprintf(w, `inductive %s : %s → Prop where
  | base (s : %s) : %s s → %s s
  | step (s s' : %s) : %s s → %s s s' → %s s'

`, reachable, st, st, init, reachable, st, reachable, side.Qualify("step"), reachable)
	return writeRelation(w, side, "tauStep", states, ir.TauEdges())
}

func dotted(ctor string) string { return "." + ctor }

func writeStates(w io.Writer, side obligationir.Side, ss []obligationir.State) {
	io.WriteString(w, "inductive "+side.Qualify("St")+" where\n")
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

func writeSymbols(w io.Writer, ds []obligationir.Declaration) {
	for _, d := range ds {
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

func writeAxioms(w io.Writer, axioms []logic.Tree) ([]string, error) {
	names := make([]string, len(axioms))
	for i, a := range axioms {
		names[i] = obligationir.AxiomName(i + 1)
		f, err := Notation.SpellFormula(a)
		if err != nil {
			return nil, fmt.Errorf("the axiom %s: %w", names[i], err)
		}
		fmt.Fprintf(w, "def %s : Prop :=\n  %s\n\n", names[i], f)
	}
	return names, nil
}

func writeRelation(w io.Writer, side obligationir.Side, name string, states map[string]obligationir.State, edges []obligationir.Edge) error {
	d, err := obligationir.StepDisjuncts(Notation, side, states, edges, dotted)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "def %s (s s' : %s) : Prop :=\n  %s\n\n", side.Qualify(name), side.Qualify("St"), d)
	return nil
}
