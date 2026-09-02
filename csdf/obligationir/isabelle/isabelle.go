// Package isabelle compiles the livelock-freedom obligation IR to an Isabelle/HOL
// proof obligation skeleton. The opaque guard/post/init predicates become
// True-placeholder definitions (Isabelle has no "opaque" keyword), each preceded by a
// comment holding the original natural-language text, so a human or LLM can fill in
// the real predicate body and discharge the theorem.
package isabelle

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

func CompileLivelockFree(w io.Writer, r io.Reader) error {
	input, err := io.ReadAll(r)
	d, err := csdf.ParseBytes(input)
	if err != nil {
		return fmt.Errorf("isabelle.Compile: %w", err)
	}
	WriteLivelockFree(w, obligationir.BuildLivelockFree(d))
	return nil
}

func CompileLivelockFreeString(input string) (string, error) {
	d, err := csdf.Parse(input)
	if err != nil {
		return "", fmt.Errorf("isabelle.Compile: %w", err)
	}
	var b strings.Builder
	WriteLivelockFree(&b, obligationir.BuildLivelockFree(d))
	return b.String(), nil
}

func MustCompileLivelockFreeString(input string) string {
	s, err := CompileLivelockFreeString(input)
	if err != nil {
		panic(err.Error())
	}
	return s
}

// WriteLivelockFree writes an Isabelle/HOL obligation skeleton for ir to w.
func WriteLivelockFree(w io.Writer, ir obligationir.IRLivelockFree) error {
	io.WriteString(w, `theory Livelock_Obligation
  imports Main
begin`)
	WriteNewLine(w, 2)

	if ir.Structurally {
		io.WriteString(w, `(* Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation. *)`)
		WriteNewLine(w, 2)
	} else {
		side := obligationir.SideSingle

		if obligationir.HasVars(ir.States) {
			io.WriteString(w, ValPrelude)
			WriteNewLine(w, 2)
		}

		if err := WriteStateTypeDeclaration(w, side, obligationir.SortIRStates(ir.States)); err != nil {
			return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
		}
		WriteNewLine(w, 2)

		if err := WritePredicates(w, ir.Predicates); err != nil {
			return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
		}

		if err := WriteTransitionSystem(w, side, ir.Init, ir.States, ir.Edges, ir.Predicates); err != nil {
			return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
		}

		WriteLivelockTheorem(w, side, "livelock_free")
		WriteNewLine(w, 2)
	}
	io.WriteString(w, `end`)
	WriteNewLine(w, 1)
	return nil
}

// WritePredicates writes the opaque-predicate placeholder layer, sorted by id.
// It is shared by every side of an obligation: pred_<id> is never side-qualified,
// so a predicate occurring in two diagrams is declared (and filled) once.
func WritePredicates(w io.Writer, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	for _, p := range obligationir.Predicates(m) {
		if err := WritePredicate(w, p); err != nil {
			return fmt.Errorf("isabelle.WritePredicates: %w", err)
		}
		WriteNewLine(w, 2)
	}
	return nil
}

// WriteTransitionSystem writes one side's operational layer: the init alias, the
// per-edge guard/post aliases, and the step, reachable and tau_step relations.
func WriteTransitionSystem(
	w io.Writer,
	side obligationir.Side,
	init obligationir.IRInit,
	states map[csdf.StateID]obligationir.IRState,
	edges []obligationir.IREdge,
	m map[obligationir.IRPredicateID]obligationir.IRPredicate,
) error {
	if err := WriteInit(w, side, init, m); err != nil {
		return fmt.Errorf("isabelle.WriteTransitionSystem: %w", err)
	}
	WriteNewLine(w, 2)

	// Every edge, not only the τ ones: the step relation that reachable is
	// built from has to include the visible transitions too.
	for _, e := range edges {
		if err := WriteEdge(w, side, e, m); err != nil {
			return fmt.Errorf("isabelle.WriteTransitionSystem: %w", err)
		}
		WriteNewLine(w, 2)
	}

	if err := WriteRelations(w, side, init, states, edges, m); err != nil {
		return fmt.Errorf("isabelle.WriteTransitionSystem: %w", err)
	}
	return nil
}

// WriteRelations writes the step, reachable and tau_step relations of one side,
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
	if err := WriteRelation(w, side, "step", states, edges, m); err != nil {
		return fmt.Errorf("isabelle.WriteRelations: %w", err)
	}
	WriteNewLine(w, 2)

	if err := WriteReachable(w, side, init, states); err != nil {
		return fmt.Errorf("isabelle.WriteRelations: %w", err)
	}
	WriteNewLine(w, 2)

	if err := WriteRelation(w, side, "tau_step", states, obligationir.TauEdges(edges), m); err != nil {
		return fmt.Errorf("isabelle.WriteRelations: %w", err)
	}
	WriteNewLine(w, 2)
	return nil
}

// WriteLivelockTheorem writes the well-foundedness obligation for one side.
// It is restricted to the reachable states: over all of st the property is
// strictly stronger than livelock freedom, so a diagram that is livelock free can
// still fail it on valuations the diagram can never enter.
func WriteLivelockTheorem(w io.Writer, side obligationir.Side, name string) {
	io.WriteString(w, `theorem `)
	io.WriteString(w, name)
	io.WriteString(w, `: "wf_on {s. `)
	io.WriteString(w, side.Qualify("reachable"))
	io.WriteString(w, ` s} {(s', s). `)
	io.WriteString(w, side.Qualify("tau_step"))
	io.WriteString(w, ` s s'}"`)
	WriteNewLine(w, 1)
	io.WriteString(w, `  oops`)
}

func WritePredicate(w io.Writer, p obligationir.IRPredicateWithID) error {
	if err := WriteLineComment(w, NewConstWriter(string(p.Predicate.Text))); err != nil {
		return fmt.Errorf("isabelle.WritePredicate: %w", err)
	}
	WriteNewLine(w, 1)
	if err := WriteDefinition(
		w,
		NewWritePredicateNameWithIDFunc("pred_", p.ID),
		NewWriteArgTypeFunc(p.Predicate.Args),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(p.Predicate.Args),
		NewConstWriter("True"),
		len(p.Predicate.Args),
		len(p.Predicate.Args),
	); err != nil {
		return fmt.Errorf("isabelle.WritePredicate: %w", err)
	}
	return nil
}

// WriteInit writes the init alias of the start edge's post predicate, which
// constrains the start state's variables and so seeds the reachable predicate.
func WriteInit(w io.Writer, side obligationir.Side, init obligationir.IRInit, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	post := m[init.Post]
	if err := WriteLineComment(w, NewConstWriter(string(post.Text))); err != nil {
		return fmt.Errorf("isabelle.WriteInit: comment: %w", err)
	}
	WriteNewLine(w, 1)
	if err := WriteDefinition(
		w,
		NewConstWriter(side.Qualify("init")),
		NewWriteArgTypeFunc(init.PostArgs),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(nil),
		NewWritePredicateNameWithIDFunc("pred_", init.Post),
		len(init.PostArgs),
		0,
	); err != nil {
		return fmt.Errorf("isabelle.WriteInit: %w", err)
	}
	return nil
}

// WriteReachable writes the inductive predicate holding of every state the
// diagram can actually enter: the start state under init, closed under step.
func WriteReachable(w io.Writer, side obligationir.Side, init obligationir.IRInit, states map[csdf.StateID]obligationir.IRState) error {
	start, ok := states[init.Dst]
	if !ok {
		return fmt.Errorf("isabelle.WriteReachable: start state %q does not exist", init.Dst)
	}

	// where goes on the continuation line as in definition, and the alternatives
	// are indented like the datatype constructors.
	io.WriteString(w, `inductive `)
	io.WriteString(w, side.Qualify("reachable"))
	io.WriteString(w, ` :: "`)
	io.WriteString(w, side.Qualify("st"))
	io.WriteString(w, ` \<Rightarrow> bool"`)
	WriteNewLine(w, 1)
	io.WriteString(w, `  where base: "`)
	io.WriteString(w, side.Qualify("init"))
	for _, f := range start.Fields {
		io.WriteString(w, ` `)
		WriteField(w, f, false)
	}
	io.WriteString(w, ` \<Longrightarrow> `)
	io.WriteString(w, side.Qualify("reachable"))
	io.WriteString(w, ` `)
	if len(start.Fields) > 0 {
		io.WriteString(w, `(`)
	}
	WriteStatePattern(w, side, init.Dst, start, false)
	if len(start.Fields) > 0 {
		io.WriteString(w, `)`)
	}
	io.WriteString(w, `"`)
	WriteNewLine(w, 1)
	// The rules are named base/step rather than start/next: "next" is an Isar
	// keyword and cannot be a rule name.
	reachable := side.Qualify("reachable")
	io.WriteString(w, `  | step: "`)
	io.WriteString(w, reachable)
	io.WriteString(w, ` s \<Longrightarrow> `)
	io.WriteString(w, side.Qualify("step"))
	io.WriteString(w, ` s s' \<Longrightarrow> `)
	io.WriteString(w, reachable)
	io.WriteString(w, ` s'"`)
	return nil
}

func WriteEdge(w io.Writer, side obligationir.Side, tau obligationir.IREdge, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	guard := m[tau.Guard]
	if err := WriteLineComment(w, NewConstWriter(string(guard.Text))); err != nil {
		return fmt.Errorf("isabelle.WriteEdge: pre-guard comment: %w", err)
	}
	WriteNewLine(w, 1)
	if err := WriteDefinition(
		w,
		NewConstWriter(side.GuardName(tau.Line)),
		NewWriteArgTypeFunc(tau.GuardArgs),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(nil),
		NewWritePredicateNameWithIDFunc("pred_", tau.Guard),
		len(tau.GuardArgs),
		0,
	); err != nil {
		return fmt.Errorf("isabelle.WriteEdge: %w", err)
	}

	WriteNewLine(w, 2)

	post := m[tau.Post]
	if err := WriteLineComment(w, NewConstWriter(string(post.Text))); err != nil {
		return fmt.Errorf("isabelle.WriteEdge: pre-post comment: %w", err)
	}
	WriteNewLine(w, 1)
	if err := WriteDefinition(
		w,
		NewConstWriter(side.PostName(tau.Line)),
		NewWriteArgTypeFunc(tau.PostArgs),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(nil),
		NewWritePredicateNameWithIDFunc("pred_", tau.Post),
		len(tau.PostArgs),
		0,
	); err != nil {
		return fmt.Errorf("isabelle.WriteEdge: %w", err)
	}
	return nil
}

// WriteRelation writes a transition relation over edges as a disjunction. It
// backs both step (every edge) and tau_step (the τ edges only), which differ in
// name and edge list alone.
func WriteRelation(w io.Writer, side obligationir.Side, name string, states map[csdf.StateID]obligationir.IRState, edges []obligationir.IREdge, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	if err := WriteDefinition(
		w,
		NewConstWriter(side.Qualify(name)),
		func(w io.Writer, _ int) error {
			io.WriteString(w, side.Qualify("st"))
			return nil
		},
		NewConstWriter("bool"),
		func(w io.Writer, i int) error {
			switch i {
			case 0:
				io.WriteString(w, "s")
			case 1:
				io.WriteString(w, "s'")
			default:
				panic(fmt.Sprintf("isabelle.WriteRelation: index out of range: %d", i))
			}
			return nil
		},
		func(w io.Writer) error {
			switch len(edges) {
			case 0:
				io.WriteString(w, `False`)
			case 1:
				WriteDisjunct(w, side, edges[0], states, m)
			default:
				for i, e := range edges {
					if i == 0 {
						io.WriteString(w, `(`)
					} else {
						WriteNewLine(w, 1)
						io.WriteString(w, `    \<or> (`)
					}
					WriteDisjunct(w, side, e, states, m)
					io.WriteString(w, `)`)
				}
			}
			return nil
		},
		2,
		2,
	); err != nil {
		return fmt.Errorf("isabelle.WriteRelation: %w", err)
	}
	return nil
}

func WriteDisjunct(w io.Writer, side obligationir.Side, e obligationir.IREdge, states map[csdf.StateID]obligationir.IRState, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
	src := states[e.Src]
	dst := states[e.Dst]

	if len(src.Fields) > 0 || len(dst.Fields) > 0 {
		io.WriteString(w, `\<exists>`)

		first := true
		for _, f := range src.Fields {
			if !first {
				io.WriteString(w, ` `)
			}
			first = false
			io.WriteString(w, f.Name)
		}
		for _, f := range dst.Fields {
			if !first {
				io.WriteString(w, ` `)
			}
			first = false
			io.WriteString(w, f.Name)
			io.WriteString(w, `'`)
		}
		io.WriteString(w, `. `)
	}
	io.WriteString(w, `s = `)
	WriteStatePattern(w, side, e.Src, src, false)
	io.WriteString(w, ` \<and> s' = `)
	WriteStatePattern(w, side, e.Dst, dst, true)
	io.WriteString(w, ` \<and> `)
	io.WriteString(w, side.GuardName(e.Line))
	for _, arg := range e.GuardArgs {
		io.WriteString(w, ` `)
		WriteArgName(w, arg)
	}
	io.WriteString(w, ` \<and> `)
	io.WriteString(w, side.PostName(e.Line))
	for _, arg := range e.PostArgs {
		io.WriteString(w, ` `)
		WriteArgName(w, arg)
	}
}

func WriteStatePattern(w io.Writer, side obligationir.Side, id csdf.StateID, st obligationir.IRState, primed bool) {
	io.WriteString(w, side.Ctor(id))
	for _, f := range st.Fields {
		io.WriteString(w, ` `)
		WriteField(w, f, primed)
	}
}

func WriteField(w io.Writer, f obligationir.IRField, primed bool) {
	io.WriteString(w, f.Name)
	if primed {
		io.WriteString(w, `'`)
	}
}

func WriteStateTypeDeclaration(w io.Writer, side obligationir.Side, ss []obligationir.IRStateWithID) error {
	if err := WriteDatatype(
		w,
		NewConstWriter(side.Qualify("st")),
		func(w io.Writer, i int) error {
			io.WriteString(w, side.Ctor(ss[i].StateID))
			return nil
		},
		func(w io.Writer, i, j int) error {
			io.WriteString(w, "val")
			return nil
		},
		func(n int) bool {
			return len(ss[n].Fields) > 0
		},
		func(w io.Writer, n int) error {
			WriteVarTypesCommentContent(w, ss[n].Fields)
			return nil
		},
		func(n int) int {
			return len(ss[n].Fields)
		},
		len(ss),
	); err != nil {
		return fmt.Errorf("isabelle.WriteStateTypeDeclaration: writeDatatype: %w", err)
	}
	return nil
}

const ValPrelude = `datatype val = ValInt int
  | ValString string
  | ValBool bool
  | ValArray "val list"
  | ValDict "(string \<times> val) list"`

func WriteVarTypesCommentContent(w io.Writer, fs []obligationir.IRField) {
	io.WriteString(w, `type:`)
	for _, f := range fs {
		io.WriteString(w, ` (`)
		io.WriteString(w, f.Name)
		io.WriteString(w, ` :: `)
		if f.Type == "" {
			io.WriteString(w, "any")
		} else {
			io.WriteString(w, f.Type)
		}
		io.WriteString(w, `)`)
	}
}

func WritePredicateID(w io.Writer, id obligationir.IRPredicateID) {
	io.WriteString(w, obligationir.FormatPredicateID(id))
}

func NewConstWriter(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		io.WriteString(w, s)
		return nil
	}
}

func NewWritePredicateNameWithIDFunc(prefix string, id obligationir.IRPredicateID) func(io.Writer) error {
	return func(w io.Writer) error {
		io.WriteString(w, prefix)
		WritePredicateID(w, id)
		return nil
	}
}

func NewWriteArgTypeFunc(args []obligationir.IRArg) func(io.Writer, int) error {
	return func(w io.Writer, _ int) error {
		io.WriteString(w, "val")
		return nil
	}
}

func NewWriteArgNameFunc(args []obligationir.IRArg) func(io.Writer, int) error {
	return func(w io.Writer, i int) error {
		arg := args[i]
		WriteArgName(w, arg)
		return nil
	}
}

func WriteArgName(w io.Writer, arg obligationir.IRArg) {
	io.WriteString(w, arg.Name)
	if arg.Primed {
		io.WriteString(w, `'`)
	}
}

func WriteLineComment(
	w io.Writer,
	writeContent func(io.Writer) error,
) error {
	io.WriteString(w, `(* `)
	if err := writeContent(w); err != nil {
		return fmt.Errorf("isabelle.WriteLineComment: %w", err)
	}
	io.WriteString(w, ` *)`)
	return nil
}

func WriteDefinition(
	w io.Writer,
	writeNameFunc func(io.Writer) error,
	writeArgTypeFunc func(io.Writer, int) error,
	writeRetTypeFunc func(io.Writer) error,
	writeArgNameFunc func(io.Writer, int) error,
	writeBodyFunc func(io.Writer) error,
	nArgTypes int,
	nArgNames int,
) error {
	if nArgTypes < 0 {
		panic(fmt.Sprintf("isabelle.WriteDefinition: nArgTypes must be: 1 <= %d", nArgTypes))
	}
	if nArgNames > nArgTypes {
		panic(fmt.Sprintf("isabelle.WriteDefinition: nArgNames must be: nArgTypes %d >= nArgNames %d", nArgTypes, nArgNames))
	}

	io.WriteString(w, `definition `)
	if err := writeNameFunc(w); err != nil {
		return fmt.Errorf("isabelle.WriteDefinition: writeNameFunc[0]: %w", err)
	}
	io.WriteString(w, ` :: "`)
	for i := range nArgTypes {
		if err := writeArgTypeFunc(w, i); err != nil {
			return fmt.Errorf("isabelle.WriteDefinition: writeArgType[%d]: %w", i, err)
		}
		io.WriteString(w, ` \<Rightarrow> `)
	}
	if err := writeRetTypeFunc(w); err != nil {
		return fmt.Errorf("isabelle.WriteDefinition: writeRetType: %w", err)
	}
	io.WriteString(w, `"
  where "`)
	if err := writeNameFunc(w); err != nil {
		return fmt.Errorf("isabelle.WriteDefinition: writeNameFunc[1]: %w", err)
	}
	io.WriteString(w, ` `)
	for i := range nArgNames {
		if err := writeArgNameFunc(w, i); err != nil {
			return fmt.Errorf("isabelle.WriteDefinition: writeArgName[%d]: %w", i, err)
		}
		io.WriteString(w, ` `)
	}
	io.WriteString(w, `\<equiv> `)
	if err := writeBodyFunc(w); err != nil {
		return fmt.Errorf("isabelle.WriteDefinition: writeBody: %w", err)
	}
	io.WriteString(w, `"`)
	return nil
}

func WriteDatatype(
	w io.Writer,
	writeTypeName func(io.Writer) error,
	writeCtor func(io.Writer, int) error,
	writeVar func(io.Writer, int, int) error,
	hasCtorComment func(int) bool,
	writeCtorComment func(io.Writer, int) error,
	nVar func(int) int,
	nCtor int,
) error {
	if nCtor < 1 {
		panic(fmt.Sprintf("isabelle.WriteDatatype: nCtor must be greater than 1 but: %d", nCtor))
	}

	io.WriteString(w, `datatype `)
	if err := writeTypeName(w); err != nil {
		return fmt.Errorf("isabelle.WriteDatatype: writeTypeName: %w", err)
	}
	for i := range nCtor {
		if i == 0 {
			io.WriteString(w, ` = `)
		} else {
			WriteNewLine(w, 1)
			io.WriteString(w, `  | `)
		}
		if err := writeCtor(w, i); err != nil {
			return fmt.Errorf("isabelle.WriteDatatype: writeCtor[%d]: %w", i, err)
		}
		for j := range nVar(i) {
			io.WriteString(w, ` `)
			if err := writeVar(w, i, j); err != nil {
				return fmt.Errorf("isabelle.WriteDatatype: writeVar[%d]: %w", i, err)
			}
		}
		if hasCtorComment(i) {
			io.WriteString(w, ` `)
			if err := WriteLineComment(w, func(w io.Writer) error {
				return writeCtorComment(w, i)
			}); err != nil {
				return fmt.Errorf("isabelle.WriteDatatype: writeComment[%d]: %w", i, err)
			}
		}
	}
	return nil
}

func WriteNewLine(w io.Writer, n int) {
	for range n {
		io.WriteString(w, "\n")
	}
}
