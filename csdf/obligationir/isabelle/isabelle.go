// Package isabelle compiles the livelock-freedom obligation IR to an Isabelle/HOL
// proof obligation skeleton. The opaque guard/post/init predicates become
// True-placeholder definitions (Isabelle has no "opaque" keyword), each preceded by a
// comment holding the original natural-language text, so a human or LLM can fill in
// the real predicate body and discharge the theorem.
package isabelle

import (
	"fmt"
	"io"
	"slices"
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
func WriteLivelockFree(w io.Writer, ir obligationir.IRLivelockFree) {
	io.WriteString(w, `theory Livelock_Obligation
  imports Main
begin

`)

	ps := ir.CollectPredicates()
	slices.SortFunc(ps, obligationir.ComparePredicate)
	hs := obligationir.IRPredicatesWithHash(ps)

	taus := make([]obligationir.IREdge, 0, len(ir.Edges))
	for _, e := range ir.Edges {
		if e.Event == csdf.Tau {
			taus = append(taus, e)
		}
	}

	if hasVars(ir) {
		io.WriteString(w, ValPrelude)
	}
	io.WriteString(w, `datatype st = `)
	stateIDs := obligationir.SortIRStates(ir.States)
	for i, st := range stateIDs {
		if i == 0 {
			io.WriteString(w, string(st.StateID))
		} else {
			io.WriteString(w, `
            | `)
			io.WriteString(w, string(st.StateID))
		}
		for range st.Fields {
			io.WriteString(w, ` val`)
		}
		if len(st.Fields) > 0 {
			io.WriteString(w, ` `)
			WriteDeclaredComment(w, st.Fields)
		}
	}
	io.WriteString(w, `

`)

	if len(taus) > 0 {
		for _, h := range hs {
			if h.Predicate.Kind == obligationir.IRPredicateKindInit {
				// NOTE: init always do not get used in tau_step.
				continue
			}

			io.WriteString(w, `(* `)
			io.WriteString(w, string(h.Predicate.Text))
			io.WriteString(w, ` *)
`)
			io.WriteString(w, `definition `)
			WritePredicateName(w, h)
			io.WriteString(w, ` :: "`)
			for range len(h.Predicate.Args) {
				io.WriteString(w, `val \<Rightarrow> `)
			}
			io.WriteString(w, `bool"
  where "`)
			WritePredicateName(w, h)
			io.WriteString(w, ` `)
			for _, arg := range h.Predicate.Args {
				WriteArg(w, arg)
				io.WriteString(w, ` `)
			}
			io.WriteString(w, `\<equiv> True"

`)
		}
	}

	WriteTauStep(w, ir, hs, taus)
	io.WriteString(w, `
`)

	if ir.Structurally {
		io.WriteString(w, `(* Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation. *)
`)
	} else {
		io.WriteString(w, `theorem livelock_free: "wf {(s', s). tau_step s s'}"
`)
		io.WriteString(w, `  oops
`)
	}
	io.WriteString(w, `end
`)
}

// WriteTauStep renders the tau_step relation as a disjunction over the tau edges. With no
// tau edge the relation is False; a single disjunct is emitted inline, several are
// parenthesised and joined with ∨ (each ∃ would otherwise capture the disjunction).
func WriteTauStep(w io.Writer, ir obligationir.IRLivelockFree, hs []obligationir.IRPredicateWithHash, taus []obligationir.IREdge) {
	predicates := ir.CollectPredicates()
	preds := make(map[string]obligationir.IRPredicate, len(predicates))
	var b strings.Builder
	for _, h := range hs {
		WritePredicateName(&b, h)
		sym := b.String()
		(&b).Reset()
		preds[sym] = h.Predicate
	}

	m := make(map[int]map[obligationir.IRPredicateKind]obligationir.IRPredicateWithHash, len(hs))
	for _, h := range hs {
		var ok bool
		var m2 map[obligationir.IRPredicateKind]obligationir.IRPredicateWithHash
		if m2, ok = m[h.Predicate.Line]; !ok {
			m2 = make(map[obligationir.IRPredicateKind]obligationir.IRPredicateWithHash)
			m[h.Predicate.Line] = m2
		}
		m2[h.Predicate.Kind] = h

	}

	io.WriteString(w, `definition tau_step :: "st \<Rightarrow> st \<Rightarrow> bool"
  where `)
	switch len(taus) {
	case 0:
		io.WriteString(w, `"tau_step s s' \<equiv> False"
`)
	case 1:
		io.WriteString(w, `"tau_step s s' \<equiv> `)

		WriteTauDisjunct(w, taus[0], ir.States, m)
		io.WriteString(w, `"
`)
	default:
		io.WriteString(w, `"tau_step s s' \<equiv>
`)
		for i, tau := range taus {
			if i == 0 {
				io.WriteString(w, `    (`)
			} else {
				io.WriteString(w, `
     \<or> (`)
			}
			WriteTauDisjunct(w, tau, ir.States, m)
			io.WriteString(w, `)`)
		}
		io.WriteString(w, `"
`)
	}
}

func WriteTauDisjunct(w io.Writer, e obligationir.IREdge, states map[csdf.StateID]obligationir.IRState, m map[int]map[obligationir.IRPredicateKind]obligationir.IRPredicateWithHash) {
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
	io.WriteString(w, ` \<and> `)
	WriteApplyPred(w, m[e.Guard.Line][obligationir.IRPredicateKindGuard])
	io.WriteString(w, ` \<and> `)
	WriteApplyPred(w, m[e.Post.Line][obligationir.IRPredicateKindPost])
}

// statePattern renders a constructor application like "a n" (or "a n'" for the primed
// post-state), or just "a" when the state has no variables.
func WriteStatePattern(w io.Writer, ctor csdf.StateID, st obligationir.IRState, primed bool) {
	io.WriteString(w, string(ctor))
	for _, f := range st.Fields {
		io.WriteString(w, ` `)
		io.WriteString(w, f.Name)
		if primed {
			io.WriteString(w, `'`)
		}
	}
}

// applyPred renders a predicate symbol applied to its arguments, or the literal True
// when the predicate was omitted.
func WriteApplyPred(w io.Writer, h obligationir.IRPredicateWithHash) {
	WritePredicateName(w, h)
	for _, a := range h.Predicate.Args {
		io.WriteString(w, ` `)
		WriteArg(w, a)
	}
}

func WriteArg(w io.Writer, a obligationir.IRArg) {
	io.WriteString(w, a.Name)
	if a.Primed {
		io.WriteString(w, `'`)
	}
}

// jsonPrelude is the value type of every state variable: csdfrepl state-var values are
// arbitrary JSON, so each variable is a json. Floats are folded into JSONInt for now.
const ValPrelude = `datatype val = ValInt int
             | ValString string
			 | ValBool bool
			 | ValArray "val list"
			 | ValDict "(string \<times> val) list"

`

// hasVars reports whether any state has a variable, in which case the json datatype is
// emitted (otherwise it would be unused).
func hasVars(ir obligationir.IRLivelockFree) bool {
	for _, st := range ir.States {
		if len(st.Fields) > 0 {
			return true
		}
	}
	return false
}

// declaredComment renders the state's original declared variable types, positionally and
// comma-joined (an undeclared field shows as "any"). It returns "" when nothing was
// declared, so no comment is emitted.
func WriteDeclaredComment(w io.Writer, fs []obligationir.IRField) {
	io.WriteString(w, `(* declared:`)
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
	io.WriteString(w, ` *)`)
}

func WritePredicateName(w io.Writer, h obligationir.IRPredicateWithHash) {
	switch h.Predicate.Kind {
	case obligationir.IRPredicateKindInit:
		io.WriteString(w, "init_")
	case obligationir.IRPredicateKindGuard:
		io.WriteString(w, "guard_")
	case obligationir.IRPredicateKindPost:
		io.WriteString(w, "post_")
	default:
		panic(fmt.Sprintf("isabelle.WritePredicateName: unknown kind: %#v", h))
	}

	io.WriteString(w, strconv.FormatUint(uint64(h.Hash), 36))
}
