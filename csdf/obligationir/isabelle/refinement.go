package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// sideIR pairs one diagram of the obligation with the names its declarations get.
type sideIR struct {
	Side obligationir.Side
	IR   obligationir.IRSide
	// Proc is the name of the top-level process this side denotes.
	Proc string
	// Vars is true when either diagram has a state variable, in which case the
	// event type is layered and process names are parameterised.
	Vars bool
	// Tau is true when either diagram has a tau edge, in which case the alphabet
	// carries HTau and both top-level processes hide it.
	Tau bool
}

func sides(ir obligationir.IRRefinement) []sideIR {
	vars, tau := hasVars(ir), hasTau(ir)
	return []sideIR{
		{Side: obligationir.SideSpec, IR: ir.Spec, Proc: "SpecProc", Vars: vars, Tau: tau},
		{Side: obligationir.SideImpl, IR: ir.Impl, Proc: "ImplProc", Vars: vars, Tau: tau},
	}
}

// hasTau reports whether either diagram has a tau edge.
func hasTau(ir obligationir.IRRefinement) bool {
	for _, s := range []obligationir.IRSide{ir.Spec, ir.Impl} {
		if len(obligationir.TauEdges(s.Edges)) > 0 {
			return true
		}
	}
	return false
}

// hasVars reports whether any state of either diagram carries a variable. The
// valuation layer (the val datatype, the Internal index and the replicated
// internal choices) exists only then.
func hasVars(ir obligationir.IRRefinement) bool {
	return obligationir.HasVars(ir.Spec.States) || obligationir.HasVars(ir.Impl.States)
}

// EventTerm is the process term for performing e. With valuations in play the
// event type is layered, so a visible event is wrapped.
func (s sideIR) EventTerm(e csdf.Event) string {
	if s.Vars {
		return "Alphabet " + obligationir.EventCtor(e)
	}
	return obligationir.EventCtor(e)
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

	if hasVars(ir) {
		io.WriteString(w, ValPrelude)
		WriteNewLine(w, 2)
	}

	if err := WriteEventDatatype(w, ir.Alphabet, hasVars(ir), hasTau(ir)); err != nil {
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
//
// With valuations in play the type is layered. CSP-Prover's replicated internal
// choice can only be indexed by the event type (its proc datatype has no spare
// type variable), and the only way in is Rep_int_choice_f's injection
// 'b => 'a. Internal is that injection's target: no process ever performs one, so
// no trace and no refusal changes, but it lets the choice range over valuations.
func WriteEventDatatype(w io.Writer, alphabet []csdf.Event, vars, tau bool) error {
	name := "event"
	if vars {
		name = "alphabet"
	}

	ctors := make([]string, 0, len(alphabet)+1)
	for _, e := range alphabet {
		ctors = append(ctors, obligationir.EventCtor(e))
	}
	if tau {
		// Not part of the visible alphabet, so it goes last rather than into the
		// sorted union.
		ctors = append(ctors, obligationir.TauCtor)
	}

	if len(ctors) == 0 {
		// A datatype needs at least one constructor, and a process over an empty
		// alphabet can only ever be STOP or SKIP.
		io.WriteString(w, `datatype `)
		io.WriteString(w, name)
		io.WriteString(w, ` = Ev_none (* neither diagram has an event *)`)
	} else if err := WriteDatatype(
		w,
		NewConstWriter(name),
		func(w io.Writer, i int) error {
			io.WriteString(w, ctors[i])
			return nil
		},
		func(w io.Writer, _, _ int) error { return nil },
		func(int) bool { return false },
		func(io.Writer, int) error { return nil },
		func(int) int { return 0 },
		len(ctors),
	); err != nil {
		return fmt.Errorf("isabelle.WriteEventDatatype: %w", err)
	}

	if !vars {
		return nil
	}

	WriteNewLine(w, 2)
	io.WriteString(w, `(* Internal only indexes the replicated internal choices below, via
   Rep_int_choice_f; no process performs it, so it changes no trace and no
   refusal. The injections into it are of the form \<lambda>(x, y). Internal [x, y],
   whose inj side condition is discharged by (simp add: inj_def). *)
datatype event = Alphabet alphabet
  | Internal "val list"`)
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

	if s.End != nil {
		if err := WriteEnd(w, side, *s.End, m); err != nil {
			return fmt.Errorf("isabelle.WriteSideAliases: %w", err)
		}
		WriteNewLine(w, 2)
	}
	return nil
}

// WriteEnd writes the alias of the end edge's guard. It is named after its own
// source line, like any other edge's.
func WriteEnd(w io.Writer, side obligationir.Side, end obligationir.IREnd, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	guard := m[end.Guard]
	if err := WriteLineComment(w, NewConstWriter(string(guard.Text))); err != nil {
		return fmt.Errorf("isabelle.WriteEnd: comment: %w", err)
	}
	WriteNewLine(w, 1)
	if err := WriteDefinition(
		w,
		NewConstWriter(side.GuardName(end.Line)),
		NewWriteArgTypeFunc(guard.Args),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(nil),
		NewWritePredicateNameWithIDFunc("pred_", end.Guard),
		len(guard.Args),
		0,
	); err != nil {
		return fmt.Errorf("isabelle.WriteEnd: %w", err)
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
			for _, f := range st.Fields {
				io.WriteString(w, ` `)
				WriteField(w, f, false)
			}
			io.WriteString(w, `) =`)
			WriteStateBody(w, s, st.StateID, ir.Predicates)
			io.WriteString(w, `"`)
		}
	}
	return nil
}

// stateBodyIndent indents every line of a process body under its primrec
// equation, so the branches of a state line up whatever their number.
const stateBodyIndent = `     `

func writeBodyLine(w io.Writer, s string) {
	WriteNewLine(w, 1)
	io.WriteString(w, stateBodyIndent)
	io.WriteString(w, s)
}

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
			writeBodyLine(w, `[+]`)
		}
		branches++
		WriteEdgeBranch(w, s, e, m)
	}

	// The end edge is CSP successful termination, offered alongside the state's
	// other out-edges and guarded like them.
	if end := s.IR.End; end != nil && end.Src == id {
		if branches > 0 {
			writeBodyLine(w, `[+]`)
		}
		branches++

		writeBodyLine(w, `(IF `)
		io.WriteString(w, application(s.Side.GuardName(end.Line), m[end.Guard].Args))
		writeBodyLine(w, ` THEN SKIP`)
		writeBodyLine(w, ` ELSE STOP)`)
	}

	if branches == 0 {
		// A state with no out-edges offers nothing and never terminates.
		writeBodyLine(w, `STOP`)
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
	dst := s.IR.States[e.Dst]
	post := s.Side.PostName(e.Line)
	postApp := application(post, m[e.Post].Args)

	writeBodyLine(w, `(IF (`)
	io.WriteString(w, application(s.Side.GuardName(e.Line), m[e.Guard].Args))
	io.WriteString(w, ` \<and> `)
	if len(dst.Fields) > 0 {
		// The post predicate has to be satisfiable, not merely written: an edge
		// whose Post no valuation satisfies cannot fire and must contribute a
		// refusal. Guarding with Guard alone would admit phantom transitions.
		io.WriteString(w, `(`)
		WriteExists(w, dst.Fields)
		io.WriteString(w, postApp)
		io.WriteString(w, `)`)
	} else {
		io.WriteString(w, postApp)
	}
	io.WriteString(w, `)`)

	writeBodyLine(w, ` THEN `)
	io.WriteString(w, s.EventTerm(e.Event))
	io.WriteString(w, ` -> `)
	WriteSuccessor(w, s, e.Dst, dst.Fields, true, postApp)

	writeBodyLine(w, ` ELSE STOP)`)
}

// fieldArgs views a state's fields as unprimed predicate arguments.
func fieldArgs(fields []obligationir.IRField) []obligationir.IRArg {
	args := make([]obligationir.IRArg, 0, len(fields))
	for _, f := range fields {
		args = append(args, obligationir.IRArg{Name: f.Name, Type: f.Type})
	}
	return args
}

// application renders `name a1 a2 ...`.
func application(name string, args []obligationir.IRArg) string {
	var b strings.Builder
	b.WriteString(name)
	for _, arg := range args {
		b.WriteString(" ")
		WriteArgName(&b, arg)
	}
	return b.String()
}

// WriteExists writes the binder quantifying over a state's post-state valuation.
func WriteExists(w io.Writer, fields []obligationir.IRField) {
	io.WriteString(w, `\<exists>`)
	for i, f := range fields {
		if i > 0 {
			io.WriteString(w, ` `)
		}
		WriteField(w, f, true)
	}
	io.WriteString(w, `. `)
}

// WriteSuccessor writes the process entered after an edge fires. When the target
// state carries variables the post predicate generally admits several valuations
// and the diagram picks one the environment cannot influence, which is a
// replicated internal choice; when it carries none the successor is determined.
func WriteSuccessor(w io.Writer, s sideIR, dst csdf.StateID, fields []obligationir.IRField, primed bool, setBody string) {
	if len(fields) == 0 {
		io.WriteString(w, `$(`)
		io.WriteString(w, s.Side.Ctor(dst))
		io.WriteString(w, `)`)
		return
	}

	pattern := valuationPattern(fields, primed)
	io.WriteString(w, `(!<\<lambda>`)
	io.WriteString(w, pattern)
	io.WriteString(w, `. Internal [`)
	for i, f := range fields {
		if i > 0 {
			io.WriteString(w, `, `)
		}
		WriteField(w, f, primed)
	}
	io.WriteString(w, `]> `)
	io.WriteString(w, pattern)
	io.WriteString(w, `:{`)
	io.WriteString(w, pattern)
	io.WriteString(w, `. `)
	io.WriteString(w, setBody)
	io.WriteString(w, `} .. $(`)
	io.WriteString(w, s.Side.Ctor(dst))
	for _, f := range fields {
		io.WriteString(w, ` `)
		WriteField(w, f, primed)
	}
	io.WriteString(w, `))`)
}

// valuationPattern is the binder a valuation is bound by: a single variable, or a
// tuple when the state carries several.
func valuationPattern(fields []obligationir.IRField, primed bool) string {
	var b strings.Builder
	if len(fields) > 1 {
		b.WriteString("(")
	}
	for i, f := range fields {
		if i > 0 {
			b.WriteString(", ")
		}
		WriteField(&b, f, primed)
	}
	if len(fields) > 1 {
		b.WriteString(")")
	}
	return b.String()
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
			start := s.IR.States[s.IR.Init.Dst]

			if s.Tau {
				// Hidden once, outermost: hiding is what turns the HTau prefixes
				// into internal steps, and hiding them per state would not compose.
				// A replicated internal choice already brings its own parentheses.
				parens := len(start.Fields) == 0
				if parens {
					io.WriteString(w, `(`)
				}
				defer func() {
					if parens {
						io.WriteString(w, `)`)
					}
					io.WriteString(w, ` -- {`)
					io.WriteString(w, s.EventTerm(csdf.Tau))
					io.WriteString(w, `}`)
				}()
			}

			if len(start.Fields) == 0 {
				io.WriteString(w, `IF `)
				io.WriteString(w, s.Side.Qualify("init"))
				io.WriteString(w, ` THEN $(`)
				io.WriteString(w, s.Side.Ctor(s.IR.Init.Dst))
				io.WriteString(w, `) ELSE STOP`)
				return nil
			}
			// The start edge's post denotes a set of initial valuations, and the
			// diagram picks one internally. An unsatisfiable one leaves the empty
			// set, whose replicated internal choice is STOP, i.e. a diagram that
			// cannot start does nothing.
			WriteSuccessor(w, s, s.IR.Init.Dst, start.Fields, false,
				application(s.Side.Qualify("init"), fieldArgs(start.Fields)))
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
