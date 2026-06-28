// Package lean compiles the livelock-freedom obligation IR to a Lean 4 proof
// obligation skeleton. The opaque guard/post/init predicates become True-placeholder
// definitions, each preceded by a comment holding the original natural-language text,
// so a human or LLM can fill in the real predicate body and discharge the theorem.
package lean

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// Compile writes a Lean 4 obligation skeleton for ir to w.
func Compile(w io.Writer, ir obligationir.ObligationIR) error {
	var b strings.Builder

	fmt.Fprintf(&b, "-- structurally_livelock_free: %t\n", ir.StructurallyLivelockFree)

	if hasVars(ir) {
		b.WriteString(jsonPrelude)
	}
	b.WriteString("inductive St where\n")
	for _, st := range ir.States {
		b.WriteString("  | " + st.Ctor)
		for _, f := range st.Fields {
			fmt.Fprintf(&b, " (%s : Json)", f.Name)
		}
		if c := declaredComment(st); c != "" {
			b.WriteString(" -- declared: " + c)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	for _, p := range ir.Predicates {
		fmt.Fprintf(&b, "-- %q\n", sanitizeComment(p.Text))
		b.WriteString("def " + p.Sym)
		for _, a := range p.Args {
			fmt.Fprintf(&b, " (%s : Json)", argName(a))
		}
		b.WriteString(" : Prop := True\n")
	}
	if len(ir.Predicates) > 0 {
		b.WriteString("\n")
	}

	b.WriteString(tauStep(ir))
	b.WriteString("\n")

	if ir.StructurallyLivelockFree {
		b.WriteString("-- Livelock freedom holds structurally: no reachable tau-cycle. No proof obligation.\n")
	} else {
		b.WriteString("theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by\n")
		b.WriteString("  sorry\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// tauStep renders the tau-step relation as a disjunction over the tau edges. With no
// tau edge the relation is False; a single disjunct is emitted bare, several are
// parenthesised and joined with ∨ (each ∃ would otherwise capture the disjunction).
func tauStep(ir obligationir.ObligationIR) string {
	states := make(map[string]obligationir.IRState, len(ir.States))
	for _, st := range ir.States {
		states[st.Ctor] = st
	}
	preds := make(map[string]obligationir.IRPredicate, len(ir.Predicates))
	for _, p := range ir.Predicates {
		preds[p.Sym] = p
	}

	var disjuncts []string
	for _, e := range ir.Edges {
		if e.Tau {
			disjuncts = append(disjuncts, tauDisjunct(e, states, preds))
		}
	}

	if len(disjuncts) == 0 {
		return "def tauStep (s s' : St) : Prop := False\n"
	}
	var b strings.Builder
	b.WriteString("def tauStep (s s' : St) : Prop :=\n")
	if len(disjuncts) == 1 {
		b.WriteString("  " + disjuncts[0] + "\n")
		return b.String()
	}
	for i, d := range disjuncts {
		if i == 0 {
			b.WriteString("  (" + d + ")\n")
		} else {
			b.WriteString("  ∨ (" + d + ")\n")
		}
	}
	return b.String()
}

func tauDisjunct(e obligationir.IREdge, states map[string]obligationir.IRState, preds map[string]obligationir.IRPredicate) string {
	src := states[e.Src]
	dst := states[e.Dst]

	var binders []string
	for _, f := range src.Fields {
		binders = append(binders, f.Name)
	}
	for _, f := range dst.Fields {
		binders = append(binders, f.Name+"'")
	}

	var b strings.Builder
	if len(binders) > 0 {
		b.WriteString("∃ " + strings.Join(binders, " ") + ", ")
	}
	b.WriteString("s = " + statePattern(e.Src, src, false))
	b.WriteString(" ∧ s' = " + statePattern(e.Dst, dst, true))
	b.WriteString(" ∧ " + applyPred(e.Guard, preds))
	b.WriteString(" ∧ " + applyPred(e.Post, preds))
	return b.String()
}

// statePattern renders an anonymous-constructor pattern like ".a n" (or ".a n'" for
// the primed post-state), or just ".a" when the state has no variables.
func statePattern(ctor string, st obligationir.IRState, primed bool) string {
	var b strings.Builder
	b.WriteString("." + ctor)
	for _, f := range st.Fields {
		if primed {
			b.WriteString(" " + f.Name + "'")
		} else {
			b.WriteString(" " + f.Name)
		}
	}
	return b.String()
}

// applyPred renders a predicate symbol applied to its arguments, or the literal True
// when the predicate was omitted.
func applyPred(sym string, preds map[string]obligationir.IRPredicate) string {
	if sym == "True" {
		return "True"
	}
	p, ok := preds[sym]
	if !ok {
		return sym
	}
	var b strings.Builder
	b.WriteString(sym)
	for _, a := range p.Args {
		b.WriteString(" " + argName(a))
	}
	return b.String()
}

func argName(a obligationir.IRArg) string {
	if a.Primed {
		return a.Name + "'"
	}
	return a.Name
}

// jsonPrelude is the value type of every state variable: csdfrepl state-var values are
// arbitrary JSON, so each variable is a Json. Floats are folded into JSONInt for now.
const jsonPrelude = `inductive Json where
  | JSONInt (i : Int)
  | JSONString (s : String)
  | JSONBool (b : Bool)
  | JSONArray (xs : List Json)
  | JSONDict (kvs : List (String × Json))

`

// hasVars reports whether any state has a variable, in which case the Json datatype is
// emitted (otherwise it would be unused).
func hasVars(ir obligationir.ObligationIR) bool {
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
func declaredComment(st obligationir.IRState) string {
	if len(st.Fields) == 0 {
		return ""
	}
	declared := false
	parts := make([]string, len(st.Fields))
	for i, f := range st.Fields {
		if f.Type != "" {
			declared = true
			parts[i] = f.Type
		} else {
			parts[i] = "any"
		}
	}
	if !declared {
		return ""
	}
	return strings.Join(parts, ", ")
}

// sanitizeComment collapses newlines so a multi-line predicate text stays on one
// Lean line comment.
func sanitizeComment(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	}), " ")
}
