// Package isabelle compiles the livelock-freedom obligation IR to an Isabelle/HOL
// proof obligation skeleton. The opaque guard/post/init predicates become
// True-placeholder definitions (Isabelle has no "opaque" keyword), each preceded by a
// comment holding the original natural-language text, so a human or LLM can fill in
// the real predicate body and discharge the theorem.
package isabelle

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
		if obligationir.HasVars(ir) {
			io.WriteString(w, ValPrelude)
			WriteNewLine(w, 2)
		}

		if err := WriteStateTypeDeclaration(w, obligationir.SortIRStates(ir.States)); err != nil {
			return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
		}
		WriteNewLine(w, 2)

		for i, p := range obligationir.OnlyPredicateWithUsedID(ir.Predicates, ir.UsedMap()) {
			if i > 0 {
				WriteNewLine(w, 2)
			}
			if err := WritePredicate(w, p); err != nil {
				return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
			}
		}
		WriteNewLine(w, 2)

		taus := obligationir.TauEdges(ir.Edges)
		for i, tau := range taus {
			if i > 0 {
				WriteNewLine(w, 2)
			}
			if err := WriteEdge(w, tau, ir.Predicates); err != nil {
				return fmt.Errorf("isabelle.WriteLivelockFree: %w", err)
			}
		}
		WriteNewLine(w, 2)

		WriteTauStep(w, ir.States, taus, ir.Predicates)
		WriteNewLine(w, 2)

		io.WriteString(w, `theorem livelock_free: "wf {(s', s). tau_step s s'}"
  oops`)
		WriteNewLine(w, 2)
	}
	io.WriteString(w, `end`)
	WriteNewLine(w, 1)
	return nil
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

func WriteEdge(w io.Writer, tau obligationir.IREdge, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	guard := m[tau.Guard]
	if err := WriteLineComment(w, NewConstWriter(string(guard.Text))); err != nil {
		return fmt.Errorf("isabelle.WriteEdge: pre-guard comment: %w", err)
	}
	WriteNewLine(w, 1)
	if err := WriteDefinition(
		w,
		NewWritePredicateNameWithLineFunc("guard_L", tau.Line),
		NewWriteArgTypeFunc(guard.Args),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(nil),
		NewWritePredicateNameWithIDFunc("pred_", tau.Guard),
		len(guard.Args),
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
		NewWritePredicateNameWithLineFunc("post_L", tau.Line),
		NewWriteArgTypeFunc(post.Args),
		NewConstWriter("bool"),
		NewWriteArgNameFunc(nil),
		NewWritePredicateNameWithIDFunc("pred_", tau.Post),
		len(post.Args),
		0,
	); err != nil {
		return fmt.Errorf("isabelle.WriteEdge: %w", err)
	}
	return nil
}

func WriteTauStep(w io.Writer, states map[csdf.StateID]obligationir.IRState, taus []obligationir.IREdge, m map[obligationir.IRPredicateID]obligationir.IRPredicate) error {
	if err := WriteDefinition(
		w,
		NewConstWriter("tau_step"),
		func(w io.Writer, _ int) error {
			io.WriteString(w, "st")
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
				panic(fmt.Sprintf("isabelle.WriteTauStep: index out of range: %d", i))
			}
			return nil
		},
		func(w io.Writer) error {
			switch len(taus) {
			case 0:
				io.WriteString(w, `False`)
			case 1:
				WriteTauDisjunct(w, taus[0], states, m)
			default:
				for i, tau := range taus {
					if i == 0 {
						io.WriteString(w, `(`)
					} else {
						WriteNewLine(w, 1)
						io.WriteString(w, `    \<or> (`)
					}
					WriteTauDisjunct(w, tau, states, m)
					io.WriteString(w, `)`)
				}
			}
			return nil
		},
		2,
		2,
	); err != nil {
		return fmt.Errorf("isabelle.WriteTauStep: %w", err)
	}
	return nil
}

func WriteTauDisjunct(w io.Writer, e obligationir.IREdge, states map[csdf.StateID]obligationir.IRState, m map[obligationir.IRPredicateID]obligationir.IRPredicate) {
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
	WriteStatePattern(w, e.Src, src, false)
	io.WriteString(w, ` \<and> s' = `)
	WriteStatePattern(w, e.Src, src, true)
	io.WriteString(w, ` \<and> guard_L`)
	WriteLineNumber(w, e.Line)
	for _, arg := range m[e.Guard].Args {
		io.WriteString(w, ` `)
		WriteArgName(w, arg)
	}
	io.WriteString(w, ` \<and> post_L`)
	WriteLineNumber(w, e.Line)
	for _, arg := range m[e.Post].Args {
		io.WriteString(w, ` `)
		WriteArgName(w, arg)
	}
}

func WriteStatePattern(w io.Writer, ctor csdf.StateID, st obligationir.IRState, primed bool) {
	io.WriteString(w, string(ctor))
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

func WriteStateTypeDeclaration(w io.Writer, ss []obligationir.IRStateWithID) error {
	if err := WriteDatatype(
		w,
		NewConstWriter("st"),
		func(w io.Writer, i int) error {
			io.WriteString(w, string(ss[i].StateID))
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
	io.WriteString(w, strconv.FormatUint(uint64(id), 36))
}

func WriteLineNumber(w io.Writer, line int) {
	io.WriteString(w, strconv.Itoa(line))
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

func NewWritePredicateNameWithLineFunc(prefix string, line int) func(io.Writer) error {
	return func(w io.Writer) error {
		io.WriteString(w, prefix)
		io.WriteString(w, strconv.Itoa(line))
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
