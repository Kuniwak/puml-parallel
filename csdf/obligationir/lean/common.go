package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// ValPrelude declares Val, the type of every value. Floats are folded into
// ValInt for now. A map is a ValMap, a list of its entries, and no other
// value is one, so a map and a value that is not one are told apart
// (docs/CLAUSE_DESIGN.md).
const ValPrelude = `inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))
  | ValMap (kvs : List (Val × Val))`

// PredicateTODO marks a declaration standing for a predicate nobody has
// formalised. It is spelled identically by every backend so that a checker can
// refuse to call an obligation discharged while any marker remains.
const PredicateTODO = `TODO(csdf): not formalised; this predicate is uninterpreted.`

// AssumptionTODO marks a declaration standing for an assumption nobody has
// formalised, spelled like PredicateTODO so that a checker refusing an
// undischarged obligation catches it too.
const AssumptionTODO = `TODO(csdf): not formalised; this assumption is uninterpreted.`

// AssumptionNames lists the propositions standing for a side's assumptions,
// in the order the diagram states them.
func AssumptionNames(side obligationir.Side, as []obligationir.Assumption) []string {
	names := make([]string, 0, len(as))
	for _, a := range as {
		names = append(names, side.AssumptionName(a.ID))
	}
	return names
}

// WriteAssumptions declares the proposition each assume note stands for. Like
// a predicate it is opaque, not True: an assumption taken for granted would
// discharge the theorem for a diagram it does not hold of.
func WriteAssumptions(w io.Writer, side obligationir.Side, as []obligationir.Assumption) error {
	for _, a := range as {
		_, err := fmt.Fprintf(w, "-- %s\n-- %s\nopaque %s : Prop\n\n", AssumptionTODO, SanitizeComment(string(a.Text)), side.AssumptionName(a.ID))
		if err != nil {
			return fmt.Errorf("lean.WriteAssumptions: %w", err)
		}
	}
	return nil
}

// WriteHypotheses writes the binders that put the named propositions in scope
// of a theorem: " (h_<name> : <name>)" for each.
func WriteHypotheses(w io.Writer, names []string) {
	for _, name := range names {
		io.WriteString(w, ` (h_`)
		io.WriteString(w, name)
		io.WriteString(w, ` : `)
		io.WriteString(w, name)
		io.WriteString(w, `)`)
	}
}

// WriteLivelockTheorem writes the well-foundedness obligation for one side.
// It is restricted to the reachable states: over all of St the property is
// strictly stronger than livelock freedom, so a diagram that is livelock free can
// still fail it on valuations the diagram can never enter. Reachable is closed
// under step, so guarding the source alone matches Isabelle's wf_on without
// needing Mathlib's Set.WellFoundedOn.
func WriteLivelockTheorem(w io.Writer, side obligationir.Side, name string, hypotheses []string) {
	io.WriteString(w, `theorem `)
	io.WriteString(w, name)
	WriteHypotheses(w, hypotheses)
	io.WriteString(w, ` :
    WellFounded (fun s' s => `)
	io.WriteString(w, side.Qualify("Reachable"))
	io.WriteString(w, ` s ∧ `)
	io.WriteString(w, side.Qualify("tauStep"))
	io.WriteString(w, ` s s') := by
  sorry`)
}

// Namespace keeps the generated declarations out of the library's, which already
// has an event type of its own. It matches the isabelle backend's theory name.
const Namespace = `Refinement_Obligation`

// refinementImport names the model the obligation is stated in. lean-csp-prover
// provides T and F only; failures-divergences reduces to F plus a
// divergence-freedom obligation per side.
func refinementImport(mode obligationir.IRRefinementMode) string {
	if mode == obligationir.IRRefinementModeTrace {
		return `LeanCspProver.CSP_T.CSP_T_Main`
	}
	return `LeanCspProver.CSP_F.CSP_F_Main`
}

// WriteRefinementTheorem writes the obligation itself, under the hypotheses
// that stand for both sides' assumptions.
func WriteRefinementTheorem(w io.Writer, mode obligationir.IRRefinementMode, hypotheses []string) {
	if mode == obligationir.IRRefinementModeTrace {
		io.WriteString(w, `-- Every trace of the Impl diagram is a trace of the Spec diagram: refT P1 M1 M2 P2
-- unfolds to semTf P2 M2 <= semTf P1 M1.
theorem refines_t`)
		WriteHypotheses(w, hypotheses)
		io.WriteString(w, ` : refTfix SpecProc ImplProc := by
  sorry`)
		return
	}

	if mode == obligationir.IRRefinementModeFailuresDivergence {
		io.WriteString(w, `-- lean-csp-prover has no FD model, so this is the standard reduction: for
-- divergence-free processes, FD refinement coincides with the F refinement.
-- Divergence of SpecProc/ImplProc arises from an infinite HTau run of the
-- underlying diagram or from a side that cannot start at all, which the
-- well-foundedness and initialisability obligations above rule out respectively.
`)
	}

	io.WriteString(w, `-- Every stable failure of the Impl diagram is one of the Spec diagram: refF
-- P1 M1 M2 P2 unfolds to semFf P2 M2 <= semFf P1 M1, and it subsumes trace
-- inclusion.
theorem refines_f`)
	WriteHypotheses(w, hypotheses)
	io.WriteString(w, ` : refFfix SpecProc ImplProc := by
  sorry`)
}

// SanitizeComment collapses newlines so a multi-line predicate text stays on one
// Lean line comment.
func SanitizeComment(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	}), " ")
}

// WriteProcFunInstances registers procfun as the library's PNfun for PN,
// which is how process-name references resolve, and picks the fixed-point
// mode. Both are typeclass instances rather than Isabelle's overloading blocks.
func WriteProcFunInstances(w io.Writer) {
	io.WriteString(w, `instance Set_procfun : HasPNfun PN event where
  PNfun := procfun

instance Set_FPmode : HasFPmode where
  FPmode := fpmode.CMSmode`)
}
