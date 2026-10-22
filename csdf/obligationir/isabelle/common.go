package isabelle

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// ValPrelude declares val, the type of every value. A map is a ValMap, a
// list of its entries, and no other value is one, so a map and a value that
// is not one are told apart (docs/CLAUSE_DESIGN.md).
const ValPrelude = `datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"
  | ValMap "(val \<times> val) list"`

// PredicateTODO marks a declaration standing for a predicate nobody has
// formalised. It is spelled identically by every backend so that a checker can
// refuse to call an obligation discharged while any marker remains.
const PredicateTODO = `TODO(csdf): not formalised; this predicate is uninterpreted.`

// AssumptionTODO marks a declaration standing for an assumption nobody has
// formalised, spelled like PredicateTODO so that a checker refusing an
// undischarged obligation catches it too.
const AssumptionTODO = `TODO(csdf): not formalised; this assumption is uninterpreted.`

// AssumptionNames lists the constants standing for a side's assumptions, in
// the order the diagram states them.
func AssumptionNames(side obligationir.Side, as []obligationir.Assumption) []string {
	names := make([]string, 0, len(as))
	for _, a := range as {
		names = append(names, side.AssumptionName(a.ID))
	}
	return names
}

// WriteAssumesShows writes what stands between a theorem's name and its
// statement: a space when there is no hypothesis, and otherwise an "assumes"
// line naming every hypothesis followed by "shows", each on its own line.
func WriteAssumesShows(w io.Writer, hypotheses []string) {
	if len(hypotheses) == 0 {
		io.WriteString(w, ` `)
		return
	}
	WriteNewLine(w, 1)
	io.WriteString(w, `  assumes `)
	for i, name := range hypotheses {
		if i > 0 {
			io.WriteString(w, ` and `)
		}
		io.WriteString(w, `"`)
		io.WriteString(w, name)
		io.WriteString(w, `"`)
	}
	WriteNewLine(w, 1)
	io.WriteString(w, `  shows `)
}

// WriteLivelockTheorem writes the well-foundedness obligation for one side.
// It is restricted to the reachable states: over all of st the property is
// strictly stronger than livelock freedom, so a diagram that is livelock free can
// still fail it on valuations the diagram can never enter.
func WriteLivelockTheorem(w io.Writer, side obligationir.Side, name string, hypotheses []string) {
	io.WriteString(w, `theorem `)
	io.WriteString(w, name)
	io.WriteString(w, `:`)
	WriteAssumesShows(w, hypotheses)
	io.WriteString(w, `"wf_on {s. `)
	io.WriteString(w, side.Qualify("reachable"))
	io.WriteString(w, ` s} {(s', s). `)
	io.WriteString(w, side.Qualify("tau_step"))
	io.WriteString(w, ` s s'}"`)
	WriteNewLine(w, 1)
	io.WriteString(w, `  oops`)
}

// WriteProcFunOverloading registers procfun as CSP-Prover's PNfun for this name
// type, which is how $-references resolve. The shape follows CSP-Prover's own
// Chaosfun declaration.
func WriteProcFunOverloading(w io.Writer) {
	io.WriteString(w, `overloading Set_procfun == "PNfun :: (PN, event) pnfun"
begin
  definition "PNfun (pn::PN) == procfun pn"
end
declare Set_procfun_def [simp]`)
}

// refinementImport names the CSP-Prover model the obligation is stated in.
// CSP-Prover provides T and F only; failures-divergences reduces to F plus a
// divergence-freedom obligation per side.
func refinementImport(mode obligationir.IRRefinementMode) string {
	if mode == obligationir.IRRefinementModeTrace {
		return `CSP_T.CSP_T`
	}
	return `CSP_F.CSP_F`
}

// WriteRefinementTheorem writes the obligation itself, under the hypotheses
// that stand for both sides' assumptions.
func WriteRefinementTheorem(w io.Writer, mode obligationir.IRRefinementMode, hypotheses []string) {
	if mode == obligationir.IRRefinementModeTrace {
		io.WriteString(w, `(* Every trace of the Impl diagram is a trace of the Spec diagram: in CSP-Prover
   P <=T Q unfolds to traces Q <= traces P. *)
theorem refines_t:`)
		WriteAssumesShows(w, hypotheses)
		io.WriteString(w, `"SpecProc <=T ImplProc"
  oops`)
		return
	}

	if mode == obligationir.IRRefinementModeFailuresDivergence {
		io.WriteString(w, `(* CSP-Prover has no FD model, so this is the standard reduction: for
   divergence-free processes, FD refinement coincides with <=F. Divergence of
   SpecProc/ImplProc arises from an infinite HTau run of the underlying diagram
   or from a side that cannot start at all, which the well-foundedness and
   initialisability obligations above rule out respectively. *)
`)
	}

	io.WriteString(w, `(* Every stable failure of the Impl diagram is one of the Spec diagram: in
   CSP-Prover P <=F Q unfolds to failures Q <= failures P, and <=F subsumes
   trace inclusion. *)
theorem refines_f:`)
	WriteAssumesShows(w, hypotheses)
	io.WriteString(w, `"SpecProc <=F ImplProc"
  oops`)
}

// WriteAssumptions declares the boolean constant each assume note stands for.
// Like an atom it is uninterpreted, not True: an assumption taken for granted
// would discharge the theorem for a diagram it does not hold of.
func WriteAssumptions(w io.Writer, side obligationir.Side, as []obligationir.Assumption) error {
	for _, a := range as {
		if _, err := fmt.Fprintf(w, "%s\n%s\nconsts %s :: \"bool\"\n\n", comment(AssumptionTODO), comment(string(a.Text)), side.AssumptionName(a.ID)); err != nil {
			return fmt.Errorf("isabelle.WriteAssumptions: %w", err)
		}
	}
	return nil
}

// WriteNewLine writes n newlines.
func WriteNewLine(w io.Writer, n int) {
	for range n {
		io.WriteString(w, "\n")
	}
}
