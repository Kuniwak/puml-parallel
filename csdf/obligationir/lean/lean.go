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

	if hasUntyped(ir) {
		b.WriteString("axiom Val : Type -- placeholder for untyped state variables\n")
	}
	b.WriteString("inductive St where\n")
	for _, st := range ir.States {
		b.WriteString("  | " + st.Ctor)
		for _, f := range st.Fields {
			fmt.Fprintf(&b, " (%s : %s)", f.Name, leanType(f.Type))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	for _, p := range ir.Predicates {
		fmt.Fprintf(&b, "-- %q\n", sanitizeComment(p.Text))
		b.WriteString("def " + p.Sym)
		for _, a := range p.Args {
			fmt.Fprintf(&b, " (%s : %s)", argName(a), leanType(a.Type))
		}
		b.WriteString(" : Prop := True\n")
	}
	if len(ir.Predicates) > 0 {
		b.WriteString("\n")
	}

	b.WriteString(tauStep(ir))
	b.WriteString("\n")

	b.WriteString("theorem livelock_free : WellFounded (fun s' s => tauStep s s') := by\n")
	b.WriteString("  sorry\n")

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

// leanType maps an IR type name to its Lean spelling, passing unknown types through
// verbatim for the reader to adjust.
func leanType(t string) string {
	switch t {
	case "":
		return "Val"
	case "nat", "Nat":
		return "Nat"
	case "bool", "Bool":
		return "Bool"
	case "int", "Int":
		return "Int"
	default:
		return t
	}
}

// hasUntyped reports whether any state variable lacks a declared type, in which case
// a placeholder type is introduced so the skeleton still parses.
func hasUntyped(ir obligationir.ObligationIR) bool {
	for _, st := range ir.States {
		for _, f := range st.Fields {
			if f.Type == "" {
				return true
			}
		}
	}
	return false
}

// sanitizeComment collapses newlines so a multi-line predicate text stays on one
// Lean line comment.
func sanitizeComment(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	}), " ")
}
