// Package lean compiles the livelock-freedom obligation IR to a Lean 4 proof
// obligation skeleton. The opaque guard/post predicates become True-placeholder
// definitions, each preceded by a comment holding the original natural-language
// text, so a human or LLM can fill in the real predicate body and discharge the
// theorem. It mirrors the isabelle backend: the same predicates are emitted under
// the same names, so the two skeletons can be compared line for line.
package lean

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

func CompileLivelockFree(w io.Writer, r io.Reader) error {
	input, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("lean.CompileLivelockFree: %w", err)
	}
	d, err := csdf.ParseBytes(input)
	if err != nil {
		return fmt.Errorf("lean.CompileLivelockFree: %w", err)
	}
	if err := WriteLivelockFree(w, obligationir.BuildLivelockFree(d)); err != nil {
		return fmt.Errorf("lean.CompileLivelockFree: %w", err)
	}
	return nil
}

func CompileLivelockFreeString(input string) (string, error) {
	d, err := csdf.Parse(input)
	if err != nil {
		return "", fmt.Errorf("lean.CompileLivelockFreeString: %w", err)
	}
	var b strings.Builder
	if err := WriteLivelockFree(&b, obligationir.BuildLivelockFree(d)); err != nil {
		return "", fmt.Errorf("lean.CompileLivelockFreeString: %w", err)
	}
	return b.String(), nil
}

func MustCompileLivelockFreeString(input string) string {
	s, err := CompileLivelockFreeString(input)
	if err != nil {
		panic(err.Error())
	}
	return s
}

// WriteLivelockFree writes a Lean 4 obligation skeleton for ir to w.
func WriteLivelockFree(w io.Writer, ir obligationir.IRLivelockFree) error {
	if ir.Structurally {
		io.WriteString(w, `-- Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation.`)
		WriteNewLine(w, 1)
		return nil
	}

	side := obligationir.SideSingle

	WriteNameTable(w, obligationir.LivelockFreeNameTable(ir))

	if obligationir.HasVars(ir.States) {
		io.WriteString(w, ValPrelude)
		WriteNewLine(w, 2)
	}

	if err := WriteStateTypeDeclaration(w, side, obligationir.SortIRStates(ir.States)); err != nil {
		return fmt.Errorf("lean.WriteLivelockFree: %w", err)
	}
	WriteNewLine(w, 2)

	WritePredicates(w, ir.Predicates)

	if err := WriteTransitionSystem(w, side, ir.Init, ir.States, ir.Edges, ir.Predicates); err != nil {
		return fmt.Errorf("lean.WriteLivelockFree: %w", err)
	}

	WriteLivelockTheorem(w, side, "livelock_free")
	WriteNewLine(w, 1)
	return nil
}

// WritePredicates writes the opaque-predicate placeholder layer, sorted by id.
// It is shared by every side of an obligation: pred_<id> is never side-qualified,
// so a predicate occurring in two diagrams is declared (and filled) once.
func WritePredicates(w io.Writer, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	for _, p := range obligationir.Predicates(m) {
		WritePredicate(w, p)
		WriteNewLine(w, 2)
	}
}

// WriteTransitionSystem writes one side's operational layer: the init alias, the
// per-edge guard/post aliases, and the step, Reachable and tauStep relations.
func WriteTransitionSystem(
	w io.Writer,
	side obligationir.Side,
	init obligationir.IRInit,
	states map[csdf.StateID]obligationir.IRState,
	edges []obligationir.IREdge,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) error {
	WriteInit(w, side, init, m)
	WriteNewLine(w, 2)

	// Every edge, not only the tau ones: the step relation that Reachable is
	// built from has to include the visible transitions too.
	for _, e := range edges {
		WriteEdge(w, side, e, m)
		WriteNewLine(w, 2)
	}

	if err := WriteRelations(w, side, init, states, edges, m); err != nil {
		return fmt.Errorf("lean.WriteTransitionSystem: %w", err)
	}
	return nil
}

// WriteRelations writes the step, Reachable and tauStep relations of one side,
// which are what the well-foundedness obligation is stated over. It is separate
// from the aliases they are built from because the refinement obligation writes
// those aliases for its own process terms already.
func WriteRelations(
	w io.Writer,
	side obligationir.Side,
	init obligationir.IRInit,
	states map[csdf.StateID]obligationir.IRState,
	edges []obligationir.IREdge,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) error {
	WriteRelation(w, side, "step", states, edges, m)
	WriteNewLine(w, 2)

	if err := WriteReachable(w, side, init, states); err != nil {
		return fmt.Errorf("lean.WriteRelations: %w", err)
	}
	WriteNewLine(w, 2)

	WriteRelation(w, side, "tauStep", states, obligationir.TauEdges(edges), m)
	WriteNewLine(w, 2)
	return nil
}

// WriteLivelockTheorem writes the well-foundedness obligation for one side.
// It is restricted to the reachable states: over all of St the property is
// strictly stronger than livelock freedom, so a diagram that is livelock free can
// still fail it on valuations the diagram can never enter. Reachable is closed
// under step, so guarding the source alone matches Isabelle's wf_on without
// needing Mathlib's Set.WellFoundedOn.
func WriteLivelockTheorem(w io.Writer, side obligationir.Side, name string) {
	io.WriteString(w, `theorem `)
	io.WriteString(w, name)
	io.WriteString(w, ` :
    WellFounded (fun s' s => `)
	io.WriteString(w, side.Qualify("Reachable"))
	io.WriteString(w, ` s ∧ `)
	io.WriteString(w, side.Qualify("tauStep"))
	io.WriteString(w, ` s s') := by
  sorry`)
}

// WriteInit writes the init alias of the start edge's post predicate, which
// constrains the start state's variables and so seeds Reachable.
func WriteInit(w io.Writer, side obligationir.Side, init obligationir.IRInit, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	post := m[init.Post]
	WriteLineComment(w, string(post.Text))
	WriteNewLine(w, 1)
	WritePredicateAlias(w, side.Qualify("init"), len(init.PostArgs), init.Post)
}

// WriteReachable writes the inductive predicate holding of every state the
// diagram can actually enter: the start state under init, closed under step.
func WriteReachable(w io.Writer, side obligationir.Side, init obligationir.IRInit, states map[csdf.StateID]obligationir.IRState) error {
	start, ok := states[init.Dst]
	if !ok {
		return fmt.Errorf("lean.WriteReachable: start state %q does not exist", init.Dst)
	}

	reachable := side.Qualify("Reachable")
	st := side.Qualify(StateType)

	io.WriteString(w, `inductive `)
	io.WriteString(w, reachable)
	io.WriteString(w, ` : `)
	io.WriteString(w, st)
	io.WriteString(w, ` → Prop where`)
	WriteNewLine(w, 1)
	io.WriteString(w, `  | base`)
	if len(start.Fields) > 0 {
		io.WriteString(w, ` (`)
		for i, f := range start.Fields {
			if i > 0 {
				io.WriteString(w, ` `)
			}
			WriteField(w, f, false)
		}
		io.WriteString(w, ` : `)
		io.WriteString(w, ValType)
		io.WriteString(w, `)`)
	}
	io.WriteString(w, ` : `)
	io.WriteString(w, side.Qualify("init"))
	for _, f := range start.Fields {
		io.WriteString(w, ` `)
		WriteField(w, f, false)
	}
	io.WriteString(w, ` → `)
	io.WriteString(w, reachable)
	io.WriteString(w, ` `)
	if len(start.Fields) > 0 {
		io.WriteString(w, `(`)
	}
	WriteStatePattern(w, side, init.Dst, start, false)
	if len(start.Fields) > 0 {
		io.WriteString(w, `)`)
	}
	WriteNewLine(w, 1)
	io.WriteString(w, `  | step (s s' : `)
	io.WriteString(w, st)
	io.WriteString(w, `) : `)
	io.WriteString(w, reachable)
	io.WriteString(w, ` s → `)
	io.WriteString(w, side.Qualify("step"))
	io.WriteString(w, ` s s' → `)
	io.WriteString(w, reachable)
	io.WriteString(w, ` s'`)
	return nil
}

// WritePredicate writes the shared True placeholder a predicate stands for. The
// binder groups same-typed arguments, as Lean prefers: (n n' : Val).
func WritePredicate(w io.Writer, p obligationir.IRPredicateWithID) {
	WriteLineComment(w, string(p.Predicate.Text))
	WriteNewLine(w, 1)
	io.WriteString(w, `def pred_`)
	WritePredicateID(w, p.ID)
	if len(p.Predicate.Args) > 0 {
		io.WriteString(w, ` (`)
		for i, arg := range p.Predicate.Args {
			if i > 0 {
				io.WriteString(w, ` `)
			}
			WriteArgName(w, arg)
		}
		io.WriteString(w, ` : `)
		io.WriteString(w, ValType)
		io.WriteString(w, `)`)
	}
	io.WriteString(w, ` : Prop := True`)
}

// WriteEdge writes the guard_L<line> and post_L<line> aliases of the tau edge's
// predicates. They are eta-reduced, so the arity shows in the type only.
func WriteEdge(w io.Writer, side obligationir.Side, tau obligationir.IREdge, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	guard := m[tau.Guard]
	WriteLineComment(w, string(guard.Text))
	WriteNewLine(w, 1)
	WritePredicateAlias(w, side.GuardName(tau.Line), len(tau.GuardArgs), tau.Guard)

	WriteNewLine(w, 2)

	post := m[tau.Post]
	WriteLineComment(w, string(post.Text))
	WriteNewLine(w, 1)
	WritePredicateAlias(w, side.PostName(tau.Line), len(tau.PostArgs), tau.Post)
}

// WritePredicateAlias writes an eta-reduced alias of a pred_<id> placeholder, so
// the arity shows in the type only.
func WritePredicateAlias(w io.Writer, name string, nArgs int, id obligationir.IRPredicateID) {
	io.WriteString(w, `def `)
	io.WriteString(w, name)
	io.WriteString(w, ` : `)
	for range nArgs {
		io.WriteString(w, ValType)
		io.WriteString(w, ` → `)
	}
	io.WriteString(w, `Prop := pred_`)
	WritePredicateID(w, id)
}

// WriteRelation writes a transition relation over edges as a disjunction. It
// backs both step (every edge) and tauStep (the tau edges only), which differ in
// name and edge list alone. With no edge the relation is False; a single disjunct
// is emitted bare, several are parenthesised so neither existential captures the
// other's clause.
func WriteRelation(w io.Writer, side obligationir.Side, name string, states map[csdf.StateID]obligationir.IRState, edges []obligationir.IREdge, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	io.WriteString(w, `def `)
	io.WriteString(w, side.Qualify(name))
	io.WriteString(w, ` (s s' : `)
	io.WriteString(w, side.Qualify(StateType))
	io.WriteString(w, `) : Prop :=`)
	switch len(edges) {
	case 0:
		io.WriteString(w, ` False`)
	case 1:
		WriteNewLine(w, 1)
		io.WriteString(w, `  `)
		WriteDisjunct(w, side, edges[0], states, m)
	default:
		for i, e := range edges {
			WriteNewLine(w, 1)
			if i == 0 {
				io.WriteString(w, `  (`)
			} else {
				io.WriteString(w, `  ∨ (`)
			}
			WriteDisjunct(w, side, e, states, m)
			io.WriteString(w, `)`)
		}
	}
}

func WriteDisjunct(w io.Writer, side obligationir.Side, e obligationir.IREdge, states map[csdf.StateID]obligationir.IRState, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	src := states[e.Src]
	dst := states[e.Dst]

	if len(src.Fields) > 0 || len(dst.Fields) > 0 {
		io.WriteString(w, `∃ `)

		first := true
		for _, f := range src.Fields {
			if !first {
				io.WriteString(w, ` `)
			}
			first = false
			WriteField(w, f, false)
		}
		for _, f := range dst.Fields {
			if !first {
				io.WriteString(w, ` `)
			}
			first = false
			WriteField(w, f, true)
		}
		io.WriteString(w, `, `)
	}
	io.WriteString(w, `s = `)
	WriteStatePattern(w, side, e.Src, src, false)
	io.WriteString(w, ` ∧ s' = `)
	WriteStatePattern(w, side, e.Dst, dst, true)
	io.WriteString(w, ` ∧ `)
	io.WriteString(w, side.GuardName(e.Line))
	for _, arg := range e.GuardArgs {
		io.WriteString(w, ` `)
		WriteArgName(w, arg)
	}
	io.WriteString(w, ` ∧ `)
	io.WriteString(w, side.PostName(e.Line))
	for _, arg := range e.PostArgs {
		io.WriteString(w, ` `)
		WriteArgName(w, arg)
	}
}

// WriteStatePattern writes an anonymous-constructor pattern like ".a n" (or
// ".a n'" for the primed post-state), or just ".a" when the state has no
// variables.
func WriteStatePattern(w io.Writer, side obligationir.Side, id csdf.StateID, st obligationir.IRState, primed bool) {
	io.WriteString(w, `.`)
	io.WriteString(w, side.Ctor(id))
	for _, f := range st.Fields {
		io.WriteString(w, ` `)
		WriteField(w, f, primed)
	}
}

func WriteField(w io.Writer, f obligationir.IRField, primed bool) {
	io.WriteString(w, obligationir.Mangle(f.Name))
	if primed {
		io.WriteString(w, `'`)
	}
}

func WriteStateTypeDeclaration(w io.Writer, side obligationir.Side, ss []obligationir.IRStateWithID) error {
	if len(ss) < 1 {
		return fmt.Errorf("lean.WriteStateTypeDeclaration: no states")
	}

	io.WriteString(w, `inductive `)
	io.WriteString(w, side.Qualify(StateType))
	io.WriteString(w, ` where`)
	for _, s := range ss {
		WriteNewLine(w, 1)
		io.WriteString(w, `  | `)
		io.WriteString(w, side.Ctor(s.StateID))
		for _, f := range s.Fields {
			io.WriteString(w, ` (`)
			io.WriteString(w, obligationir.Mangle(f.Name))
			io.WriteString(w, ` : `)
			io.WriteString(w, ValType)
			io.WriteString(w, `)`)
		}
		if len(s.Fields) > 0 {
			io.WriteString(w, ` `)
			WriteLineCommentFunc(w, func(w io.Writer) {
				WriteVarTypesCommentContent(w, s.Fields)
			})
		}
	}
	return nil
}

// StateType is the state-space datatype's unqualified name.
const StateType = `St`

// ValType is the type of every state variable: csdfrepl state-var values are
// arbitrary JSON, so each variable is a Val.
const ValType = `Val`

// ValPrelude declares ValType. Floats are folded into ValInt for now.
const ValPrelude = `inductive Val where
  | ValInt (i : Int)
  | ValString (s : String)
  | ValBool (b : Bool)
  | ValArray (xs : List Val)
  | ValDict (kvs : List (String × Val))`

// WriteVarTypesCommentContent writes the state's originally declared variable
// types; an undeclared one shows as "any".
func WriteVarTypesCommentContent(w io.Writer, fs []obligationir.IRField) {
	io.WriteString(w, `type:`)
	for _, f := range fs {
		io.WriteString(w, ` (`)
		io.WriteString(w, f.Name)
		io.WriteString(w, ` : `)
		if f.Type == "" {
			io.WriteString(w, "any")
		} else {
			io.WriteString(w, f.Type)
		}
		io.WriteString(w, `)`)
	}
}

func WriteArgName(w io.Writer, arg obligationir.IRArg) {
	io.WriteString(w, obligationir.Mangle(arg.Name))
	if arg.Primed {
		io.WriteString(w, `'`)
	}
}

func WritePredicateID(w io.Writer, id obligationir.IRPredicateID) {
	io.WriteString(w, obligationir.FormatPredicateID(id))
}

func WriteLineNumber(w io.Writer, line int) {
	io.WriteString(w, strconv.Itoa(line))
}

// WriteLineComment writes s as a Lean line comment. Newlines are collapsed so a
// multi-line predicate text stays on one comment line.
func WriteLineComment(w io.Writer, s string) {
	WriteLineCommentFunc(w, func(w io.Writer) {
		io.WriteString(w, SanitizeComment(s))
	})
}

func WriteLineCommentFunc(w io.Writer, writeContent func(io.Writer)) {
	io.WriteString(w, `-- `)
	writeContent(w)
}

// SanitizeComment collapses newlines so a multi-line predicate text stays on one
// Lean line comment.
func SanitizeComment(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	}), " ")
}

func WriteNewLine(w io.Writer, n int) {
	for range n {
		io.WriteString(w, "\n")
	}
}

// WriteNameTable writes the correspondence between the identifiers of the
// generated theory and the CSDF names they encode. CSDF names are not
// identifiers, so an event like "choose(product)" has to be encoded; without the
// table the reader cannot tell which diagram element a declaration came from.
// Nothing is written when every name was already an identifier.
func WriteNameTable(w io.Writer, names []obligationir.MangledName) {
	if len(names) == 0 {
		return
	}

	WriteLineComment(w, `Names that are not Lean identifiers are encoded; the originals are:`)
	for _, n := range names {
		WriteNewLine(w, 1)
		WriteLineComment(w, `  `+n.Identifier+` = `+strconv.Quote(n.Original))
	}
	WriteNewLine(w, 2)
}
