package isabelle

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// sideIR pairs one diagram of the obligation with the names its declarations get.
type sideIR struct {
	Side obligationir.Side
	IR   obligationir.IRSide
	// Proc is the name of the top-level process this side denotes.
	Proc string
}

func sides(ir obligationir.IRRefinement) []sideIR {
	return []sideIR{
		{Side: obligationir.SideSpec, IR: ir.Spec, Proc: "SpecProc"},
		{Side: obligationir.SideImpl, IR: ir.Impl, Proc: "ImplProc"},
	}
}

// WriteRefinement writes an Isabelle/HOL refinement obligation skeleton for ir to
// w. Both diagrams are encoded as CSP-Prover process terms over one shared event
// datatype, and the obligation itself is the one-line refinement statement; the
// metatheory that makes it meaningful is CSP-Prover's, not ours.
func WriteRefinement(w io.Writer, ir obligationir.IRRefinement) error {
	io.WriteString(w, `theory Refinement_Obligation
  imports `)
	io.WriteString(w, refinementImport(ir.Mode))
	WriteNewLine(w, 1)
	io.WriteString(w, `begin`)
	WriteNewLine(w, 2)

	if err := WriteEventDatatype(w, ir.Alphabet); err != nil {
		return fmt.Errorf("isabelle.WriteRefinement: %w", err)
	}
	WriteNewLine(w, 2)

	if err := WritePredicates(w, ir.Predicates); err != nil {
		return fmt.Errorf("isabelle.WriteRefinement: %w", err)
	}

	for _, s := range sides(ir) {
		if err := WriteSideAliases(w, s.Side, s.IR, ir.Predicates); err != nil {
			return fmt.Errorf("isabelle.WriteRefinement: %w", err)
		}
	}

	if err := WriteProcessNameDatatype(w, ir); err != nil {
		return fmt.Errorf("isabelle.WriteRefinement: %w", err)
	}
	WriteNewLine(w, 2)

	if err := WriteProcFun(w, ir); err != nil {
		return fmt.Errorf("isabelle.WriteRefinement: %w", err)
	}
	WriteNewLine(w, 2)

	WriteProcFunOverloading(w)
	WriteNewLine(w, 2)

	for _, s := range sides(ir) {
		if err := WriteProcDefinition(w, s); err != nil {
			return fmt.Errorf("isabelle.WriteRefinement: %w", err)
		}
		WriteNewLine(w, 2)
	}

	WriteRefinementTheorem(w, ir.Mode)
	WriteNewLine(w, 2)

	io.WriteString(w, `end`)
	WriteNewLine(w, 1)
	return nil
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

// WriteEventDatatype writes the shared alphabet of both diagrams. Refusal
// information is relative to this one type, so both processes are typed over it.
func WriteEventDatatype(w io.Writer, alphabet []csdf.Event) error {
	if len(alphabet) == 0 {
		// A datatype needs at least one constructor, and a process over an empty
		// alphabet can only ever be STOP or SKIP.
		io.WriteString(w, `datatype event = Ev_none (* neither diagram has a visible event *)`)
		return nil
	}

	if err := WriteDatatype(
		w,
		NewConstWriter("event"),
		func(w io.Writer, i int) error {
			io.WriteString(w, obligationir.EventCtor(alphabet[i]))
			return nil
		},
		func(w io.Writer, _, _ int) error { return nil },
		func(int) bool { return false },
		func(io.Writer, int) error { return nil },
		func(int) int { return 0 },
		len(alphabet),
	); err != nil {
		return fmt.Errorf("isabelle.WriteEventDatatype: %w", err)
	}
	return nil
}

// WriteSideAliases writes one side's location-named aliases of the shared
// predicate placeholders: the init predicate and each edge's guard and post.
func WriteSideAliases(
	w io.Writer,
	side obligationir.Side,
	s obligationir.IRSide,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) error {
	if err := WriteInit(w, side, s.Init, m); err != nil {
		return fmt.Errorf("isabelle.WriteSideAliases: %w", err)
	}
	WriteNewLine(w, 2)

	for _, e := range s.Edges {
		if err := WriteEdge(w, side, e, m); err != nil {
			return fmt.Errorf("isabelle.WriteSideAliases: %w", err)
		}
		WriteNewLine(w, 2)
	}
	return nil
}

// WriteProcessNameDatatype writes the process-name datatype covering both sides:
// CSP-Prover resolves $-references through a single PNfun per name type, so the
// two diagrams have to share one datatype, under their side-qualified
// constructors.
func WriteProcessNameDatatype(w io.Writer, ir obligationir.IRRefinement) error {
	type ctor struct {
		Name string
		N    int
	}

	var ctors []ctor
	for _, s := range sides(ir) {
		for _, st := range obligationir.SortIRStates(s.IR.States) {
			ctors = append(ctors, ctor{Name: s.Side.Ctor(st.StateID), N: len(st.Fields)})
		}
	}

	if err := WriteDatatype(
		w,
		NewConstWriter("PN"),
		func(w io.Writer, i int) error {
			io.WriteString(w, ctors[i].Name)
			return nil
		},
		func(w io.Writer, _, _ int) error {
			io.WriteString(w, "val")
			return nil
		},
		func(int) bool { return false },
		func(io.Writer, int) error { return nil },
		func(i int) int { return ctors[i].N },
		len(ctors),
	); err != nil {
		return fmt.Errorf("isabelle.WriteProcessNameDatatype: %w", err)
	}
	return nil
}

// WriteProcFun writes the body of every process name: one primrec equation per
// state, whose body is the external choice over that state's out-edges. Edges
// carrying the same event collapse to an internal choice by the CSP law
// (a -> P) [+] (a -> Q) = a -> (P |~| Q), which is exactly the nondeterminism the
// diagram means; using an internal choice here instead would wrongly let the
// process refuse events the diagram offers.
func WriteProcFun(w io.Writer, ir obligationir.IRRefinement) error {
	io.WriteString(w, `primrec
  procfun :: "(PN, event) pnfun"
where`)

	first := true
	for _, s := range sides(ir) {
		for _, st := range obligationir.SortIRStates(s.IR.States) {
			WriteNewLine(w, 1)
			if first {
				io.WriteString(w, `  "`)
			} else {
				io.WriteString(w, `| "`)
			}
			first = false

			io.WriteString(w, `procfun (`)
			io.WriteString(w, s.Side.Ctor(st.StateID))
			io.WriteString(w, `) = `)
			WriteStateBody(w, s, st.StateID, ir.Predicates)
			io.WriteString(w, `"`)
		}
	}
	return nil
}

// stateBodyIndent lines up the [+] branches under the first one, which starts
// after `| "procfun (<ctor>) = `.
const stateBodyIndent = `                    `

// WriteStateBody writes the external choice over the out-edges of one state.
func WriteStateBody(
	w io.Writer,
	s sideIR,
	id csdf.StateID,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) {
	branches := 0
	for _, e := range s.IR.Edges {
		if e.Src != id {
			continue
		}
		if branches > 0 {
			WriteNewLine(w, 1)
			io.WriteString(w, stateBodyIndent)
			io.WriteString(w, `[+] `)
		}
		branches++
		WriteEdgeBranch(w, s, e, m)
	}

	if branches == 0 {
		// A state with no out-edges offers nothing and never terminates.
		io.WriteString(w, `STOP`)
	}
}

// WriteEdgeBranch writes one out-edge as a guarded prefix. The guard is the full
// enabledness condition of the edge, not just its Guard: an edge whose Post is
// unsatisfiable cannot fire and must contribute a refusal, so leaving the
// satisfiability conjunct out would admit phantom transitions.
func WriteEdgeBranch(
	w io.Writer,
	s sideIR,
	e obligationir.IREdge,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) {
	io.WriteString(w, `(IF (`)
	io.WriteString(w, s.Side.GuardName(e.Line))
	io.WriteString(w, ` \<and> `)
	io.WriteString(w, s.Side.PostName(e.Line))
	io.WriteString(w, `) THEN `)
	io.WriteString(w, obligationir.EventCtor(e.Event))
	io.WriteString(w, ` -> $(`)
	io.WriteString(w, s.Side.Ctor(e.Dst))
	io.WriteString(w, `) ELSE STOP)`)
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

// WriteProcDefinition writes the top-level process one diagram denotes: its start
// state, entered only when the start edge's post predicate admits it.
func WriteProcDefinition(w io.Writer, s sideIR) error {
	if err := WriteDefinition(
		w,
		NewConstWriter(s.Proc),
		func(io.Writer, int) error { return nil },
		NewConstWriter(`(PN, event) proc`),
		func(io.Writer, int) error { return nil },
		func(w io.Writer) error {
			io.WriteString(w, `IF `)
			io.WriteString(w, s.Side.Qualify("init"))
			io.WriteString(w, ` THEN $(`)
			io.WriteString(w, s.Side.Ctor(s.IR.Init.Dst))
			io.WriteString(w, `) ELSE STOP`)
			return nil
		},
		0,
		0,
	); err != nil {
		return fmt.Errorf("isabelle.WriteProcDefinition: %w", err)
	}
	return nil
}

// WriteRefinementTheorem writes the obligation itself.
func WriteRefinementTheorem(w io.Writer, mode obligationir.IRRefinementMode) {
	if mode == obligationir.IRRefinementModeTrace {
		io.WriteString(w, `(* Every trace of the Impl diagram is a trace of the Spec diagram: in CSP-Prover
   P <=T Q unfolds to traces Q <= traces P. *)
theorem refines_t: "SpecProc <=T ImplProc"
  oops`)
		return
	}

	io.WriteString(w, `(* Every stable failure of the Impl diagram is one of the Spec diagram: in
   CSP-Prover P <=F Q unfolds to failures Q <= failures P, and <=F subsumes
   trace inclusion. *)
theorem refines_f: "SpecProc <=F ImplProc"
  oops`)
}
