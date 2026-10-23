package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// WriteRefinement writes the refinement obligation of two diagrams
// whose clauses are formulas, for CSP-Prover. Each state of a side is a
// process name applied to its values; each edge from it is an external
// choice of the events it performs, ? ev:X -> P, of the values its event
// carries, followed by an internal choice of the values after it, so an
// edge none of whose events is enabled is STOP. The environment is declared
// as for the livelock-freedom obligation, once for both sides.
func WriteRefinement(w io.Writer, r obligationir.Refinement) error {
	return WriteRefinementEdited(w, r, Edits{})
}

// Edits are what a reader formalising an obligation supplies, written in
// place of what the generator leaves open: Types gives the body of each
// type predicate it names, over v, in place of its declaration; Proofs
// gives the proof of each theorem it names in place of the open one; and
// Lemmas are written before the end.
type Edits struct {
	Types  map[string]string
	Proofs map[string]string
	Lemmas []string
}

// WriteRefinementEdited writes the refinement obligation of r as
// WriteRefinement does, with e.
func WriteRefinementEdited(w io.Writer, r obligationir.Refinement, e Edits) error {
	text, err := refinement(r, e)
	if err != nil {
		return fmt.Errorf("isabelle.WriteRefinement: %w", err)
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

func refinement(r obligationir.Refinement, e Edits) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	var w strings.Builder
	fmt.Fprintf(&w, "theory Refinement_Obligation\n  imports %s\nbegin\n\n%s\n\n", refinementImport(r.Mode), ValPrelude)
	writeAlphabet(&w, r.Alphabet)

	ds, err := r.Declarations(PredicateTODO)
	if err != nil {
		return "", err
	}
	writeSymbols(&w, ds, e.Types)
	builtins := obligationir.BuiltinsOf(ds)
	if err := WriteBuiltins(&w, builtins); err != nil {
		return "", err
	}
	if err := WriteLemmas(&w, builtins, r.Lemmas()); err != nil {
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
	WriteProcFunOverloading(&w)
	io.WriteString(&w, "\n\n")
	for _, s := range sides {
		if err := writeProc(&w, s); err != nil {
			return "", err
		}
	}
	for _, s := range sides {
		if err := writeSideLemmas(&w, s, r.Mode, e.Proofs); err != nil {
			return "", err
		}
	}
	// The refinement is stated under no hypothesis: an assumption of the
	// impl could make it hold vacuously, and one of the spec, of liveness, is
	// no claim traces or stable failures can state (docs/CLAUSE_DESIGN.md).
	WriteRefinementTheorem(&w, r.Mode, nil)
	io.WriteString(&w, "\n\n")
	for _, l := range e.Lemmas {
		io.WriteString(&w, l+"\n\n")
	}
	io.WriteString(&w, "end\n")
	return w.String(), nil
}

// writeAlphabet writes the events of both sides: each channel applied to the
// value of each of its fields, and HTau, which hiding makes internal; then
// event, whose Internal only indexes the replicated internal choices.
func writeAlphabet(w io.Writer, alphabet []obligationir.Channel) {
	io.WriteString(w, "datatype alphabet =")
	for i, c := range alphabet {
		if i > 0 {
			io.WriteString(w, "\n  |")
		}
		io.WriteString(w, " "+obligationir.EventCtor(csdf.Event(c.Name))+strings.Repeat(" val", c.Arity))
		if obligationir.IsMangled(c.Name) {
			io.WriteString(w, " "+comment(c.Name))
		}
	}
	if len(alphabet) > 0 {
		io.WriteString(w, "\n  |")
	}
	io.WriteString(w, " "+obligationir.TauCtor+`

(* Internal only indexes the replicated internal choices below, via
   Rep_int_choice_f; no process performs it, so it changes no trace and no
   refusal. The injections into it are of the form \<lambda>(x, y). Internal [x, y],
   whose inj side condition is discharged by (simp add: inj_def). *)
datatype event = Alphabet alphabet
  | Internal "val list"

`)
}

func writeProcessNames(w io.Writer, sides []side) {
	io.WriteString(w, "datatype PN =")
	first := true
	for _, s := range sides {
		for _, st := range s.ir.States {
			if !first {
				io.WriteString(w, "\n  |")
			}
			first = false
			io.WriteString(w, " "+s.names.Ctor(csdf.StateID(st.Name))+strings.Repeat(" val", len(st.Vars)))
			if obligationir.IsMangled(st.Name) || len(st.Vars) > 0 {
				io.WriteString(w, " "+comment(st.Name+obligationir.VarsComment(st.Vars)))
			}
		}
	}
	io.WriteString(w, "\n\n")
}

func writeProcFun(w io.Writer, sides []side) error {
	io.WriteString(w, "primrec\n  procfun :: \"(PN, event) pnfun\"\nwhere")
	first := true
	for _, s := range sides {
		states := s.ir.StateMap()
		for _, st := range s.ir.States {
			if first {
				io.WriteString(w, "\n  ")
			} else {
				io.WriteString(w, "\n| ")
			}
			first = false
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
				branches = append(branches, "(IF ("+guard+") THEN SKIP ELSE STOP)")
			}
			body := "STOP"
			if len(branches) > 0 {
				body = strings.Join(branches, "\n     [+]\n     ")
			}
			fmt.Fprintf(w, "\"procfun (%s) =\n     %s\"", processName(s.names, st, false), body)
		}
	}
	io.WriteString(w, "\n\n")
	return nil
}

// processName is the process name of the state st holding its values, after
// the step when primed.
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

// branch is the process an edge is from its source: ? ev:X -> P.
func branch(s side, states map[string]obligationir.State, e obligationir.Edge) (string, error) {
	enabled, next, after, err := obligationir.EdgeSets(Notation, states, e, "ev", eventTerm(e))
	if err != nil {
		return "", err
	}
	return "(? ev:{ev. " + enabled + "} -> " + successor(s.names, states[e.Dst], after, next) + ")", nil
}

// eventTerm is the event e performs, as a value of event.
func eventTerm(e obligationir.Edge) string {
	if e.Tau() {
		return "Alphabet " + obligationir.TauCtor
	}
	ctor := obligationir.EventCtor(csdf.Event(e.Channel))
	if len(e.Fields) == 0 {
		return "Alphabet " + ctor
	}
	return "Alphabet (" + strings.Join(append([]string{ctor}, obligationir.EventArgs(e)...), " ") + ")"
}

// successor is the process that picks values of vars, which body holds of,
// and continues as dst holding them: the name of dst when it holds nothing.
func successor(names obligationir.Side, dst obligationir.State, vars []string, body string) string {
	target := "$(" + processName(names, dst, true) + ")"
	if len(vars) == 0 {
		return target
	}
	pattern := vars[0]
	if len(vars) > 1 {
		pattern = "(" + strings.Join(vars, ", ") + ")"
	}
	inj := `(\<lambda>` + pattern + `. Internal [` + strings.Join(vars, ", ") + `])`
	return `(!<` + inj + `> ` + pattern + `:{` + pattern + `. ` + body + `} .. ` + target + `)`
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
		p = "(IF (" + body + ") THEN $(" + processName(s.names, start, false) + ") ELSE DIV)"
	} else {
		p = successor(s.names, start, vars, body)
	}
	fmt.Fprintf(w, "definition %s :: \"(PN, event) proc\"\n  where \"%s \\<equiv> %s -- {Alphabet %s}\"\n\n", s.proc, s.proc, p, obligationir.TauCtor)
	return nil
}

// writeSideLemmas writes the lemmas of a side. That it can start at all is
// stated under no hypothesis, since it must not rest on what the side
// assumes of its runs: of the impl it is what keeps the refinement from
// holding vacuously, as an impl that cannot start is DIV; of the spec it is
// a sanity check. In FD, that no reachable state starts an infinite run of
// tau steps is stated under the side's own assumptions, so
// the FD conclusion is conditional on them; with the first it licenses
// reading the F refinement as FD refinement.
func writeSideLemmas(w io.Writer, s side, mode obligationir.IRRefinementMode, proofs map[string]string) error {
	assumed := AssumptionNames(s.names, s.ir.Assumptions)
	if mode == obligationir.IRRefinementModeFailuresDivergence && s.ir.Structurally {
		fmt.Fprintf(w, "(* %s is livelock free structurally: no reachable tau-cycle. *)\n\n", s.label)
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
	fmt.Fprintf(w, `(* %s admits an initial valuation, so it does not denote DIV, which
   diverges on the empty trace and, as the Impl, would make the refinement
   vacuous. Stated under no hypothesis: it must not rest on an assumption. *)
theorem %s:`, s.label, s.names.Qualify("initialisable"))
	WriteAssumesShows(w, nil)
	proof, ok := proofs[s.names.Qualify("initialisable")]
	if !ok {
		proof = "oops"
	}
	fmt.Fprintf(w, "\"%s\"\n  %s\n\n", body, proof)
	return nil
}
