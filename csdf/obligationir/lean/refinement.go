package lean

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
	// States names the side's transition-system layer, which exists only in
	// failures-divergence mode.
	States obligationir.Side
	// Label names the side in prose.
	Label string
}

func sides(ir obligationir.IRRefinement) []sideIR {
	vars, tau := hasVars(ir), hasTau(ir)
	return []sideIR{
		{
			Side: obligationir.SideSpec, States: obligationir.SideSpecStates,
			IR: ir.Spec, Proc: "SpecProc", Label: "Spec", Vars: vars, Tau: tau,
		},
		{
			Side: obligationir.SideImpl, States: obligationir.SideImplStates,
			IR: ir.Impl, Proc: "ImplProc", Label: "Impl", Vars: vars, Tau: tau,
		},
	}
}

// hasVars reports whether any state of either diagram carries a variable. The
// valuation layer (the Val datatype, the Internal index and the replicated
// internal choices) exists only then.
func hasVars(ir obligationir.IRRefinement) bool {
	return obligationir.HasVars(ir.Spec.States) || obligationir.HasVars(ir.Impl.States)
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

// NeedsLivelockObligation reports whether this side has to carry a
// divergence-freedom obligation: only in failures-divergence mode, and only when
// the structural tau-cycle check did not already settle it.
func (s sideIR) NeedsLivelockObligation() bool {
	return s.IR.StructurallyLivelockFree != nil && !*s.IR.StructurallyLivelockFree
}

// HasLivelockLayer reports whether the side carries the divergence-freedom
// reduction at all, discharged structurally or not.
func (s sideIR) HasLivelockLayer() bool {
	return s.IR.StructurallyLivelockFree != nil
}

// EventTerm is the process term for performing e. With valuations in play the
// event type is layered, so a visible event is wrapped.
func (s sideIR) EventTerm(e csdf.Event) string {
	ctor := EventType + "." + obligationir.EventCtor(e)
	if s.Vars {
		return EventType + "." + AlphabetCtor + " " + AlphabetType + "." + obligationir.EventCtor(e)
	}
	return ctor
}

// WriteRefinement writes a Lean 4 refinement obligation skeleton for ir to w.
// Both diagrams are encoded as lean-csp-prover process terms and the obligation
// is the one-line refinement statement; the metatheory that makes it meaningful
// is that library's, not ours. It mirrors the isabelle backend declaration for
// declaration, under the same names.
func WriteRefinement(w io.Writer, ir obligationir.IRRefinement) error {
	io.WriteString(w, `import `)
	io.WriteString(w, refinementImport(ir.Mode))
	WriteNewLine(w, 2)

	// The declarations below live in a namespace because the library already has
	// an event type of its own, and so would clash outside one.
	io.WriteString(w, `-- The guards below are propositions, so the Prop-valued procIte is used rather
-- than the Bool-valued proc.IF.
attribute [local instance] Classical.propDecidable

noncomputable section

namespace `)
	io.WriteString(w, Namespace)
	WriteNewLine(w, 2)

	WriteNameTable(w, obligationir.RefinementNameTable(ir))

	if hasVars(ir) {
		io.WriteString(w, ValPrelude)
		WriteNewLine(w, 1)
		io.WriteString(w, `deriving Inhabited`)
		WriteNewLine(w, 2)
	}

	if err := WriteEventDatatype(w, ir.Alphabet, hasVars(ir), hasTau(ir)); err != nil {
		return fmt.Errorf("lean.WriteRefinement: %w", err)
	}
	WriteNewLine(w, 2)

	WritePredicates(w, ir.Predicates)

	for _, s := range sides(ir) {
		WriteSideAliases(w, s.Side, s.IR, ir.Predicates)
	}

	if err := WriteProcessNameDatatype(w, ir); err != nil {
		return fmt.Errorf("lean.WriteRefinement: %w", err)
	}
	WriteNewLine(w, 2)

	if err := WriteProcFun(w, ir); err != nil {
		return fmt.Errorf("lean.WriteRefinement: %w", err)
	}
	WriteNewLine(w, 2)

	WriteProcFunInstances(w)
	WriteNewLine(w, 2)

	for _, s := range sides(ir) {
		WriteProcDefinition(w, s)
		WriteNewLine(w, 2)
	}

	for _, s := range sides(ir) {
		if err := WriteLivelockLayer(w, s, ir.Predicates); err != nil {
			return fmt.Errorf("lean.WriteRefinement: %w", err)
		}
	}

	WriteRefinementTheorem(w, ir.Mode)
	WriteNewLine(w, 2)

	io.WriteString(w, `end `)
	io.WriteString(w, Namespace)
	WriteNewLine(w, 2)
	io.WriteString(w, `end`)
	WriteNewLine(w, 1)
	return nil
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

// The event type's parts. With valuations in play the alphabet is layered under
// the event type, because a replicated internal choice can only be indexed by the
// event type and Internal is the injection's target.
const (
	EventType    = `event`
	AlphabetType = `alphabet`
	AlphabetCtor = `Alphabet`
	InternalCtor = `Internal`
)

// WriteEventDatatype writes the shared alphabet of both diagrams. Refusal
// information is relative to this one type, so both processes are typed over it.
//
// Inhabited is derived because Rep_int_choice_f, the only way to index a
// replicated internal choice by anything but an event, requires it of both types.
func WriteEventDatatype(w io.Writer, alphabet []csdf.Event, vars, tau bool) error {
	name := EventType
	if vars {
		name = AlphabetType
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
		ctors = append(ctors, `Ev_none -- neither diagram has an event`)
	}

	io.WriteString(w, `inductive `)
	io.WriteString(w, name)
	io.WriteString(w, ` where`)
	for _, c := range ctors {
		WriteNewLine(w, 1)
		io.WriteString(w, `  | `)
		io.WriteString(w, c)
	}
	WriteNewLine(w, 1)
	io.WriteString(w, `deriving Inhabited`)

	if !vars {
		return nil
	}

	WriteNewLine(w, 2)
	io.WriteString(w, `-- Internal only indexes the replicated internal choices below, via
-- Rep_int_choice_f; no process performs it, so it changes no trace and no
-- refusal. Its injections are of the form fun (x, y) => event.Internal [x, y].
inductive `)
	io.WriteString(w, EventType)
	io.WriteString(w, ` where
  | `)
	io.WriteString(w, AlphabetCtor)
	io.WriteString(w, ` (a : `)
	io.WriteString(w, AlphabetType)
	io.WriteString(w, `)
  | `)
	io.WriteString(w, InternalCtor)
	io.WriteString(w, ` (vs : List `)
	io.WriteString(w, ValType)
	io.WriteString(w, `)
deriving Inhabited`)
	return nil
}

// WriteSideAliases writes one side's location-named aliases of the shared
// predicate placeholders: the init predicate and each edge's guard and post.
func WriteSideAliases(
	w io.Writer,
	side obligationir.Side,
	s obligationir.IRSide,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) {
	WriteInit(w, side, s.Init, m)
	WriteNewLine(w, 2)

	for _, e := range s.Edges {
		WriteEdge(w, side, e, m)
		WriteNewLine(w, 2)
	}

	if s.End != nil {
		WriteEnd(w, side, *s.End, m)
		WriteNewLine(w, 2)
	}
}

// WriteEnd writes the alias of the end edge's guard. It is named after its own
// source line, like any other edge's.
func WriteEnd(w io.Writer, side obligationir.Side, end obligationir.IREnd, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	guard := m[end.Guard]
	WriteLineComment(w, string(guard.Text))
	WriteNewLine(w, 1)
	WritePredicateAlias(w, side.GuardName(end.Line), len(end.GuardArgs), end.Guard)
}

// WriteProcessNameDatatype writes the process-name datatype covering both sides:
// PNfun is resolved per name type, so the two diagrams have to share one datatype
// under their side-qualified constructors.
func WriteProcessNameDatatype(w io.Writer, ir obligationir.IRRefinement) error {
	io.WriteString(w, `inductive `)
	io.WriteString(w, ProcNameType)
	io.WriteString(w, ` where`)

	n := 0
	for _, s := range sides(ir) {
		for _, st := range obligationir.SortIRStates(s.IR.States) {
			n++
			WriteNewLine(w, 1)
			io.WriteString(w, `  | `)
			io.WriteString(w, s.Side.Ctor(st.StateID))
			for _, f := range st.Fields {
				io.WriteString(w, ` (`)
				io.WriteString(w, obligationir.Mangle(f.Name))
				io.WriteString(w, ` : `)
				io.WriteString(w, ValType)
				io.WriteString(w, `)`)
			}
		}
	}
	if n == 0 {
		return fmt.Errorf("lean.WriteProcessNameDatatype: no states")
	}

	WriteNewLine(w, 1)
	io.WriteString(w, `deriving Inhabited`)
	return nil
}

// ProcNameType is the process-name datatype's name.
const ProcNameType = `PN`

// WriteProcFun writes the body of every process name: one equation per state,
// whose body is the external choice over that state's out-edges. Edges carrying
// the same event collapse to an internal choice by the CSP law
// (a -> P) [+] (a -> Q) = a -> (P |~| Q), which is exactly the nondeterminism the
// diagram means; using an internal choice here instead would wrongly let the
// process refuse events the diagram offers.
func WriteProcFun(w io.Writer, ir obligationir.IRRefinement) error {
	io.WriteString(w, `def procfun : `)
	io.WriteString(w, ProcNameType)
	io.WriteString(w, ` → proc `)
	io.WriteString(w, ProcNameType)
	io.WriteString(w, ` `)
	io.WriteString(w, EventType)

	for _, s := range sides(ir) {
		for _, st := range obligationir.SortIRStates(s.IR.States) {
			WriteNewLine(w, 1)
			io.WriteString(w, `  | `)
			io.WriteString(w, ProcNameType)
			io.WriteString(w, `.`)
			io.WriteString(w, s.Side.Ctor(st.StateID))
			for _, f := range st.Fields {
				io.WriteString(w, ` `)
				WriteField(w, f, false)
			}
			io.WriteString(w, ` =>`)
			WriteStateBody(w, s, st.StateID, ir.Predicates)
		}
	}
	return nil
}

// stateBodyIndent indents every line of a process body under its equation, so the
// branches of a state line up whatever their number.
const stateBodyIndent = `    `

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

		writeBodyLine(w, `procIte (`)
		io.WriteString(w, application(s.Side.GuardName(end.Line), end.GuardArgs))
		io.WriteString(w, `)`)
		writeBodyLine(w, `  proc.SKIP`)
		writeBodyLine(w, `  proc.STOP`)
	}

	if branches == 0 {
		// A state with no out-edges offers nothing and never terminates.
		writeBodyLine(w, `proc.STOP`)
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
	postApp := application(s.Side.PostName(e.Line), e.PostArgs)

	writeBodyLine(w, `procIte (`)
	io.WriteString(w, application(s.Side.GuardName(e.Line), e.GuardArgs))
	io.WriteString(w, ` ∧ `)
	if len(dst.Fields) > 0 {
		WriteExists(w, dst.Fields)
		io.WriteString(w, postApp)
	} else {
		io.WriteString(w, postApp)
	}
	io.WriteString(w, `)`)

	writeBodyLine(w, `  (`)
	io.WriteString(w, s.EventTerm(e.Event))
	io.WriteString(w, ` ~> `)
	WriteSuccessor(w, s, e.Dst, dst.Fields, true, postApp)
	io.WriteString(w, `)`)

	writeBodyLine(w, `  proc.STOP`)
}

// WriteExists writes the binder quantifying over a state's post-state valuation.
func WriteExists(w io.Writer, fields []obligationir.IRField) {
	io.WriteString(w, `∃ `)
	for i, f := range fields {
		if i > 0 {
			io.WriteString(w, ` `)
		}
		WriteField(w, f, true)
	}
	io.WriteString(w, `, `)
}

// WriteSuccessor writes the process entered after an edge fires. When the target
// state carries variables the post predicate generally admits several valuations
// and the diagram picks one the environment cannot influence, which is a
// replicated internal choice; when it carries none the successor is determined.
func WriteSuccessor(w io.Writer, s sideIR, dst csdf.StateID, fields []obligationir.IRField, primed bool, setBody string) {
	if len(fields) == 0 {
		io.WriteString(w, `proc.Proc_name `)
		io.WriteString(w, ProcNameType)
		io.WriteString(w, `.`)
		io.WriteString(w, s.Side.Ctor(dst))
		return
	}

	pattern := valuationPattern(fields, primed)
	io.WriteString(w, `Rep_int_choice_f (fun `)
	io.WriteString(w, pattern)
	io.WriteString(w, ` => `)
	io.WriteString(w, EventType)
	io.WriteString(w, `.`)
	io.WriteString(w, InternalCtor)
	io.WriteString(w, ` [`)
	for i, f := range fields {
		if i > 0 {
			io.WriteString(w, `, `)
		}
		WriteField(w, f, primed)
	}
	io.WriteString(w, `]) {`)
	io.WriteString(w, pattern)
	io.WriteString(w, ` | `)
	io.WriteString(w, setBody)
	io.WriteString(w, `}`)
	WriteNewLine(w, 1)
	io.WriteString(w, stateBodyIndent)
	io.WriteString(w, `    (fun `)
	io.WriteString(w, pattern)
	io.WriteString(w, ` => proc.Proc_name (`)
	io.WriteString(w, ProcNameType)
	io.WriteString(w, `.`)
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

// WriteProcFunInstances registers procfun as the library's PNfun for this name
// type, which is how process-name references resolve, and picks the fixed-point
// mode. Both are typeclass instances rather than Isabelle's overloading blocks.
func WriteProcFunInstances(w io.Writer) {
	io.WriteString(w, `instance Set_procfun : HasPNfun `)
	io.WriteString(w, ProcNameType)
	io.WriteString(w, ` `)
	io.WriteString(w, EventType)
	io.WriteString(w, ` where
  PNfun := procfun

instance Set_FPmode : HasFPmode where
  FPmode := fpmode.CMSmode`)
}

// WriteProcDefinition writes the top-level process one diagram denotes: its start
// state, entered only when the start edge's post predicate admits it.
func WriteProcDefinition(w io.Writer, s sideIR) {
	io.WriteString(w, `def `)
	io.WriteString(w, s.Proc)
	io.WriteString(w, ` : proc `)
	io.WriteString(w, ProcNameType)
	io.WriteString(w, ` `)
	io.WriteString(w, EventType)
	io.WriteString(w, ` :=`)
	WriteNewLine(w, 1)
	io.WriteString(w, `  `)

	if s.Tau {
		// Hidden once, outermost: hiding is what turns the HTau prefixes into
		// internal steps, and hiding them per state would not compose.
		io.WriteString(w, `proc.Hiding`)
		WriteNewLine(w, 1)
		io.WriteString(w, `    (`)
	}

	start := s.IR.States[s.IR.Init.Dst]
	if len(start.Fields) == 0 {
		io.WriteString(w, `procIte `)
		io.WriteString(w, s.Side.Qualify("init"))
		io.WriteString(w, ` (proc.Proc_name `)
		io.WriteString(w, ProcNameType)
		io.WriteString(w, `.`)
		io.WriteString(w, s.Side.Ctor(s.IR.Init.Dst))
		io.WriteString(w, `) proc.STOP`)
	} else {
		// The start edge's post denotes a set of initial valuations, and the
		// diagram picks one internally. An unsatisfiable one leaves the empty set,
		// whose replicated internal choice is STOP: a diagram that cannot start
		// does nothing.
		WriteSuccessor(w, s, s.IR.Init.Dst, start.Fields, false,
			application(s.Side.Qualify("init"), fieldArgs(start.Fields)))
	}

	if s.Tau {
		io.WriteString(w, `)`)
		WriteNewLine(w, 1)
		io.WriteString(w, `    {`)
		io.WriteString(w, s.EventTerm(csdf.Tau))
		io.WriteString(w, `}`)
	}
}

// WriteLivelockLayer writes one side's divergence-freedom reduction: the state
// datatype, the transition relations and the well-foundedness obligation over
// them. It is the csdflivelockfree obligation inlined, and exists only in
// failures-divergence mode, where it is what licenses reading the F refinement as
// FD refinement.
func WriteLivelockLayer(w io.Writer, s sideIR, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	if !s.HasLivelockLayer() {
		return nil
	}

	if !s.NeedsLivelockObligation() {
		io.WriteString(w, `-- `)
		io.WriteString(w, s.Label)
		io.WriteString(w, ` is livelock free structurally: no reachable tau-cycle.`)
		WriteNewLine(w, 2)
		return nil
	}

	if err := WriteStateTypeDeclaration(w, s.States, obligationir.SortIRStates(s.IR.States)); err != nil {
		return fmt.Errorf("lean.WriteLivelockLayer: %w", err)
	}
	WriteNewLine(w, 2)

	if err := WriteRelations(w, s.States, s.IR.Init, s.IR.States, s.IR.Edges, m); err != nil {
		return fmt.Errorf("lean.WriteLivelockLayer: %w", err)
	}

	WriteLivelockTheorem(w, s.States, s.Side.Qualify("livelock_free"))
	WriteNewLine(w, 2)
	return nil
}

// WriteRefinementTheorem writes the obligation itself.
func WriteRefinementTheorem(w io.Writer, mode obligationir.IRRefinementMode) {
	if mode == obligationir.IRRefinementModeTrace {
		io.WriteString(w, `-- Every trace of the Impl diagram is a trace of the Spec diagram: refT P1 M1 M2 P2
-- unfolds to semTf P2 M2 <= semTf P1 M1.
theorem refines_t : refTfix SpecProc ImplProc := by
  sorry`)
		return
	}

	if mode == obligationir.IRRefinementModeFailuresDivergence {
		io.WriteString(w, `-- lean-csp-prover has no FD model, so this is the standard reduction: for
-- divergence-free processes, FD refinement coincides with the F refinement.
-- Divergence of SpecProc/ImplProc arises exactly from infinite HTau runs of the
-- underlying diagram, which the well-foundedness obligations above rule out.
`)
	}

	io.WriteString(w, `-- Every stable failure of the Impl diagram is one of the Spec diagram: refF
-- P1 M1 M2 P2 unfolds to semFf P2 M2 <= semFf P1 M1, and it subsumes trace
-- inclusion.
theorem refines_f : refFfix SpecProc ImplProc := by
  sorry`)
}
