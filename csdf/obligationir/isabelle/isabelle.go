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

	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

// Compile writes an Isabelle/HOL obligation skeleton for ir to w.
func Compile(w io.Writer, ir obligationir.ObligationIR) error {
	var b strings.Builder

	b.WriteString("theory Livelock_Obligation imports Main begin\n")
	fmt.Fprintf(&b, "(* structurally_livelock_free: %t *)\n", ir.StructurallyLivelockFree)

	ctors := make([]string, 0, len(ir.States))
	for _, st := range ir.States {
		c := st.Ctor
		for _, f := range st.Fields {
			c += " " + isaType(f.Type)
		}
		ctors = append(ctors, c)
	}
	b.WriteString("datatype st = " + strings.Join(ctors, " | ") + "\n")
	b.WriteString("\n")

	for _, p := range ir.Predicates {
		fmt.Fprintf(&b, "(* %s *)\n", comment(p.Text))
		sig := "bool"
		if len(p.Args) > 0 {
			types := make([]string, 0, len(p.Args))
			for _, a := range p.Args {
				types = append(types, isaType(a.Type))
			}
			sig = strings.Join(types, " ⇒ ") + " ⇒ bool"
		}
		lhs := p.Sym
		for _, a := range p.Args {
			lhs += " " + argName(a)
		}
		fmt.Fprintf(&b, "definition %s :: %q where %q\n", p.Sym, sig, lhs+" ≡ True")
	}
	if len(ir.Predicates) > 0 {
		b.WriteString("\n")
	}

	b.WriteString(tauStep(ir))
	b.WriteString("\n")

	b.WriteString("theorem livelock_free: \"wf {(s', s). tau_step s s'}\"\n")
	b.WriteString("  oops\n")
	b.WriteString("end\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// tauStep renders the tau_step relation as a disjunction over the tau edges. With no
// tau edge the relation is False; a single disjunct is emitted inline, several are
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

	var b strings.Builder
	b.WriteString("definition tau_step :: \"st ⇒ st ⇒ bool\" where\n")
	switch len(disjuncts) {
	case 0:
		b.WriteString("  \"tau_step s s' ≡ False\"\n")
	case 1:
		b.WriteString("  \"tau_step s s' ≡ " + disjuncts[0] + "\"\n")
	default:
		lines := make([]string, 0, len(disjuncts))
		for i, d := range disjuncts {
			if i == 0 {
				lines = append(lines, "    ("+d+")")
			} else {
				lines = append(lines, "    ∨ ("+d+")")
			}
		}
		b.WriteString("  \"tau_step s s' ≡\n")
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\"\n")
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
		b.WriteString("∃" + strings.Join(binders, " ") + ". ")
	}
	b.WriteString("s = " + statePattern(e.Src, src, false))
	b.WriteString(" ∧ s' = " + statePattern(e.Dst, dst, true))
	b.WriteString(" ∧ " + applyPred(e.Guard, preds))
	b.WriteString(" ∧ " + applyPred(e.Post, preds))
	return b.String()
}

// statePattern renders a constructor application like "a n" (or "a n'" for the primed
// post-state), or just "a" when the state has no variables.
func statePattern(ctor string, st obligationir.IRState, primed bool) string {
	var b strings.Builder
	b.WriteString(ctor)
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

// isaType maps an IR type name to its Isabelle/HOL spelling, passing unknown types
// through verbatim for the reader to adjust.
func isaType(t string) string {
	switch t {
	case "nat", "Nat":
		return "nat"
	case "bool", "Bool":
		return "bool"
	case "int", "Int":
		return "int"
	default:
		return t
	}
}

// comment renders a natural-language predicate text as a single quoted span safe to
// place inside an Isabelle (* ... *) comment.
func comment(s string) string {
	one := strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	}), " ")
	return strings.ReplaceAll(fmt.Sprintf("%q", one), "*)", "* )")
}
