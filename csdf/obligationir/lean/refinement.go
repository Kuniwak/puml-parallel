package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// WriteRefinement writes the refinement obligation of two diagrams
// whose clauses are formulas, for lean-csp-prover, as the Isabelle back end
// does (isabelle.WriteRefinement).
func WriteRefinement(w io.Writer, r obligationir.Refinement) error {
	text, err := refinement(r)
	if err != nil {
		return fmt.Errorf("lean.WriteRefinement: %w", err)
	}
	_, err = io.WriteString(w, text)
	return err
}

// side is one diagram of a refinement and how it is named.
type side struct {
	ir     obligationir.LivelockFree
	label  string
	proc   string
	names  obligationir.Side // its process names, S_ or I_
	states obligationir.Side // its states in the livelock layer, St_S_ or St_I_
}

func sidesOf(r obligationir.Refinement) []side {
	return []side{
		{ir: r.Spec, label: "Spec", proc: "SpecProc", names: obligationir.SideSpec, states: obligationir.SideSpecStates},
		{ir: r.Impl, label: "Impl", proc: "ImplProc", names: obligationir.SideImpl, states: obligationir.SideImplStates},
	}
}

func refinement(r obligationir.Refinement) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	var w strings.Builder
	fmt.Fprintf(&w, `import %s

-- Every name is mangled behind a prefix of its kind, and a mangled name holds
-- "__", which Mathlib's naming linter warns of.
set_option linter.style.nameCheck false

-- The guards below are propositions, so the Prop-valued procIte is used rather
-- than the Bool-valued proc.IF.
attribute [local instance] Classical.propDecidable

noncomputable section

namespace %s

%s
deriving Inhabited

`, refinementImport(r.Mode), Namespace, ValPrelude)
	writeAlphabet(&w, r.Alphabet)

	ds, err := r.Declarations(PredicateTODO)
	if err != nil {
		return "", err
	}
	writeSymbols(&w, ds)
	axioms, err := writeAxioms(&w, r.Axioms())
	if err != nil {
		return "", err
	}
	sides := sidesOf(r)
	for _, s := range sides {
		if err := WriteAssumptions(&w, s.names, s.ir.Assumptions); err != nil {
			return "", err
		}
	}

	writeProcessNames(&w, sides)
	if err := writeProcFun(&w, sides); err != nil {
		return "", err
	}
	WriteProcFunInstances(&w)
	io.WriteString(&w, "\n\n")
	for _, s := range sides {
		if err := writeProc(&w, s); err != nil {
			return "", err
		}
	}
	for _, s := range sides {
		if err := writeSideLemmas(&w, s, r.Mode, axioms); err != nil {
			return "", err
		}
	}
	// The refinement is stated under the axioms alone: an assumption of the
	// impl could make it hold vacuously, and one of the spec, of liveness, is
	// no claim traces or stable failures can state (docs/CLAUSE_DESIGN.md).
	WriteRefinementTheorem(&w, r.Mode, axioms)
	fmt.Fprintf(&w, "\n\nend %s\n\nend\n", Namespace)
	return w.String(), nil
}

// writeAlphabet writes the events of both sides: each channel applied to the
// value of each of its fields, and HTau, which hiding makes internal; then
// event, whose Internal only indexes the replicated internal choices.
func writeAlphabet(w io.Writer, alphabet []obligationir.Channel) {
	io.WriteString(w, "inductive alphabet where\n")
	for _, c := range alphabet {
		io.WriteString(w, "  | "+obligationir.EventCtor(csdf.Event(c.Name)))
		for i := range c.Arity {
			fmt.Fprintf(w, " (a%d : Val)", i+1)
		}
		if obligationir.IsMangled(c.Name) {
			io.WriteString(w, " -- "+SanitizeComment(c.Name))
		}
		io.WriteString(w, "\n")
	}
	io.WriteString(w, "  | "+obligationir.TauCtor+`
deriving Inhabited

-- Internal only indexes the replicated internal choices below, via
-- Rep_int_choice_f; no process performs it, so it changes no trace and no
-- refusal. Its injections are of the form fun (x, y) => event.Internal [x, y].
inductive event where
  | Alphabet (a : alphabet)
  | Internal (vs : List Val)
deriving Inhabited

`)
}

func writeProcessNames(w io.Writer, sides []side) {
	io.WriteString(w, "inductive PN where\n")
	for _, s := range sides {
		for _, st := range s.ir.States {
			io.WriteString(w, "  | "+s.names.Ctor(csdf.StateID(st.Name)))
			for _, v := range st.Vars {
				io.WriteString(w, " ("+obligationir.VarName(v)+" : Val)")
			}
			if obligationir.IsMangled(st.Name) || len(st.Vars) > 0 {
				io.WriteString(w, " -- "+SanitizeComment(st.Name+obligationir.VarsComment(st.Vars)))
			}
			io.WriteString(w, "\n")
		}
	}
	io.WriteString(w, "deriving Inhabited\n\n")
}

func writeProcFun(w io.Writer, sides []side) error {
	io.WriteString(w, "def procfun : PN → proc PN event")
	for _, s := range sides {
		states := s.ir.StateMap()
		for _, st := range s.ir.States {
			var branches []string
			for _, e := range s.ir.Edges {
				if e.Src != st.Name {
					continue
				}
				b, err := branch(s, states, e)
				if err != nil {
					return fmt.Errorf("%s: the edge %s -> %s: %w", s.label, e.Src, e.Dst, err)
				}
				branches = append(branches, b)
			}
			for _, e := range s.ir.Ends {
				if e.Src != st.Name {
					continue
				}
				guard, err := Notation.SpellFormula(e.Guard)
				if err != nil {
					return fmt.Errorf("%s: the end edge from %s: %w", s.label, e.Src, err)
				}
				branches = append(branches, "procIte ("+guard+") proc.SKIP proc.STOP")
			}
			body := "proc.STOP"
			if len(branches) > 0 {
				body = strings.Join(branches, "\n    [+]\n    ")
			}
			fmt.Fprintf(w, "\n  | %s =>\n    %s", "PN."+processName(s.names, st, false), body)
		}
	}
	io.WriteString(w, "\n\n")
	return nil
}

// processName is the process name of the state st applied to its values,
// after the step when primed, without the PN. that a pattern takes.
func processName(names obligationir.Side, st obligationir.State, primed bool) string {
	parts := []string{names.Ctor(csdf.StateID(st.Name))}
	for _, v := range st.Vars {
		if primed {
			v += "'"
		}
		parts = append(parts, obligationir.VarName(v))
	}
	return strings.Join(parts, " ")
}

// branch is the process an edge is from its source: an external choice of
// the events it performs.
func branch(s side, states map[string]obligationir.State, e obligationir.Edge) (string, error) {
	enabled, next, after, err := obligationir.EdgeSets(Notation, states, e, "ev", eventTerm(e))
	if err != nil {
		return "", err
	}
	return "proc.Ext_pre_choice {ev | " + enabled + "}\n      (fun ev => " + successor(s.names, states[e.Dst], after, next) + ")", nil
}

// eventTerm is the event e performs, as a value of event.
func eventTerm(e obligationir.Edge) string {
	if e.Tau() {
		return "event.Alphabet alphabet." + obligationir.TauCtor
	}
	ctor := "alphabet." + obligationir.EventCtor(csdf.Event(e.Channel))
	if len(e.Fields) == 0 {
		return "event.Alphabet " + ctor
	}
	return "event.Alphabet (" + strings.Join(append([]string{ctor}, obligationir.EventArgs(e)...), " ") + ")"
}

// successor is the process that picks values of vars, which body holds of,
// and continues as dst holding them: the name of dst when it holds nothing.
func successor(names obligationir.Side, dst obligationir.State, vars []string, body string) string {
	target := "proc.Proc_name (PN." + processName(names, dst, true) + ")"
	if len(vars) == 0 {
		return target
	}
	pattern := vars[0]
	if len(vars) > 1 {
		pattern = "(" + strings.Join(vars, ", ") + ")"
	}
	return "Rep_int_choice_f (fun " + pattern + " => event.Internal [" + strings.Join(vars, ", ") + "]) {" + pattern + " | " + body + "}\n        (fun " + pattern + " => " + target + ")"
}

// writeProc writes the process a side denotes: its start state, of values
// its start post admits, hidden HTau. An unsatisfiable start post leaves the
// empty choice, DIV, and so does a start state that holds nothing.
func writeProc(w io.Writer, s side) error {
	body, vars, err := obligationir.InitSet(Notation, s.ir)
	if err != nil {
		return fmt.Errorf("%s: %w", s.label, err)
	}
	start := s.ir.StateMap()[s.ir.Init]
	var p string
	if len(vars) == 0 {
		p = "procIte (" + body + ") (proc.Proc_name (PN." + processName(s.names, start, false) + ")) proc.DIV"
	} else {
		p = successor(s.names, start, vars, body)
	}
	fmt.Fprintf(w, "def %s : proc PN event :=\n  proc.Hiding\n    (%s)\n    {event.Alphabet alphabet.%s}\n\n", s.proc, p, obligationir.TauCtor)
	return nil
}

// writeSideLemmas writes the lemmas of a side. That it can start at all is
// stated under the axioms alone, since it must not rest on what the side
// assumes of its runs: of the impl it is what keeps the refinement from
// holding vacuously, as an impl that cannot start is DIV; of the spec it is
// a sanity check. In FD, that no reachable state starts an infinite run of
// tau steps is stated under the axioms and the side's own assumptions, so
// the FD conclusion is conditional on them; with the first it licenses
// reading the F refinement as FD refinement.
func writeSideLemmas(w io.Writer, s side, mode obligationir.IRRefinementMode, axioms []string) error {
	assumed := append(append([]string{}, axioms...), AssumptionNames(s.names, s.ir.Assumptions)...)
	if mode == obligationir.IRRefinementModeFailuresDivergence && s.ir.Structurally {
		fmt.Fprintf(w, "-- %s is livelock free structurally: no reachable tau-cycle.\n\n", s.label)
	} else if mode == obligationir.IRRefinementModeFailuresDivergence {
		writeStates(w, s.states, s.ir.States)
		if err := writeTransitionSystem(w, s.states, s.ir); err != nil {
			return fmt.Errorf("%s: %w", s.label, err)
		}
		WriteLivelockTheorem(w, s.states, s.names.Qualify("livelock_free"), assumed)
		io.WriteString(w, "\n\n")
	}
	body, vars, err := obligationir.InitSet(Notation, s.ir)
	if err != nil {
		return fmt.Errorf("%s: %w", s.label, err)
	}
	if len(vars) > 0 {
		body = Notation.Exists(vars, body)
	}
	fmt.Fprintf(w, `-- %s admits an initial valuation, so it does not denote DIV, which diverges on
-- the empty trace and, as the Impl, would make the refinement vacuous. Stated
-- under the axioms alone: it must not rest on an assumption.
theorem %s`, s.label, s.names.Qualify("initialisable"))
	WriteHypotheses(w, axioms)
	fmt.Fprintf(w, " : %s := by\n  sorry\n\n", body)
	return nil
}
