package lint

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// PostLeavesVariableFreeRule reports a post that never names a state
// variable of its target after the step. A post is a formula over v and v'
// (docs/CLAUSE_DESIGN.md); a v' it does not name is constrained by nothing,
// so after the step v may be any value of its type. That is legal, and it
// is how a nondeterministic choice is written, but it is more often a
// forgotten @eq(v', v).
//
// A variable counts as named when its primed name occurs anywhere in a term
// the post reads (core.UnnamedAfter): in an atom's hole, in a builtin (@eq,
// @frame, @update, @at, …), as a key or an argument of a function. A
// self-transition may also be reported by self-transition-post-breaks-guard;
// the two say different things: this one that the step forgets a value, that
// one that the guard may not hold again. An intended choice is said by
// binding it, `@exists(x : T) (@eq(v', x))` over the declared type T of v,
// which names v' and so silences the rule; the suggestion binds a name
// apart from those the edge reads (core.Diagram.BindersApart). An atom
// naming v' silences it too, but an atom is an environment function csdfbmc
// refuses to leave undecided, so it is never suggested. The start edge is checked as every other edge, against the
// variables of the start state. Findings are HINTs.
type PostLeavesVariableFreeRule struct{}

func (PostLeavesVariableFreeRule) ID() string { return "post-leaves-variable-free" }

func (PostLeavesVariableFreeRule) Description() string {
	return "A post that never names a variable of its target after the step lets it take any value, often a forgotten @eq(v', v)."
}

func (r PostLeavesVariableFreeRule) Check(in *Input) []Finding {
	d := in.Diagram
	if d == nil {
		return nil
	}

	var findings []Finding
	if free := core.UnnamedAfter(d.InitPost, d.Vars[d.Init]); len(free) > 0 {
		findings = append(findings, r.finding(in.File, d.InitLine, d.InitPost, free, func(post string) string {
			return fmt.Sprintf(
				"The start edge to state `%s` has the post `%s`, which does not name %s, so %s may start as any value of its type. Give it a start value in the post, such as `@eq(%[5]s, …)`; if any start value is intended, say so by binding it, such as `%[6]s`.",
				d.Init, post, quoted(primed(free), ", "), quoted(free, ", "), primed(free)[0], choice(d, free[0], d.FreshStartBinder("x")),
			)
		}))
	}
	for _, edge := range d.Edges {
		free := core.UnnamedAfter(edge.Post, d.Vars[edge.Dst])
		if len(free) == 0 {
			continue
		}
		findings = append(findings, r.finding(in.File, edge.Line, edge.Post, free, func(post string) string {
			return fmt.Sprintf(
				"The edge `%s` -> `%s` on `%s` has the post `%s`, which does not name %s, so after the step %s may be any value of its type. If it should not change, keep it in the post, such as `@eq(%[7]s, %[8]s)`; if the choice is intended, say so by binding it, such as `%[9]s`.",
				edge.Src, edge.Dst, edge.Event.Label(), post, quoted(primed(free), ", "), quoted(free, ", "), primed(free)[0], free[0],
				choice(d, free[0], d.FreshBinder(edge, "x")),
			)
		}))
	}
	return findings
}

// finding is the HINT on line for a post that leaves free free, its message
// made by message from the printed post, or the ERROR saying the post does
// not print.
func (r PostLeavesVariableFreeRule) finding(file string, line int, post clause.And, free []string, message func(post string) string) Finding {
	f := Finding{File: file, StartLine: line, EndLine: line, RuleID: r.ID(), Severity: SeverityHint}
	printed, err := clause.Print(post)
	if err != nil {
		f.Severity = SeverityError
		f.Message = fmt.Sprintf(
			"The post on this line leaves %s free but cannot be printed: %s. A diagram the reader read always prints, so this is a bug of csdflint; please report it.",
			quoted(free, ", "), err,
		)
		return f
	}
	f.Message = message(printed)
	return f
}

// choice is the post that says v takes any value of its declared type,
// @exists(x : T) (@eq(v', x)), binding x, a name the reader accepts there.
func choice(d *core.Diagram, v, x string) string {
	typ := "<type>"
	if t, err := d.VarType(v); err == nil {
		typ = t.String()
	}
	return fmt.Sprintf("@exists(%s : %s) (@eq(%s', %s))", x, typ, v, x)
}

// primed are names, each primed.
func primed(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = n + "'"
	}
	return out
}
