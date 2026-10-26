package lint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// SelfTransitionPostBreaksGuardRule reports a transition from a state back
// to itself whose post may make its own guard false. A self-transition says
// the system is in the same situation after the event as before it; if the
// post falsifies the guard, it is not - the edge is enabled once and then
// not again - and the two situations are two states written as one.
//
// Whether the post falsifies the guard depends on atoms, which are never
// evaluated here. What the skeleton decides is the converse: a post that
// keeps every variable the guard reads, by @eq(v', v) or by @frame(v', v)
// without keys, keeps the guard, since an atom reads only its holes. Such an
// edge, and one whose guard reads nothing, is not reported; every other
// self-transition is a HINT naming the variables not kept. A field of the
// event is never kept: it is sent or received anew on each step. A name
// that is neither a state variable nor a field, such as a constant, cannot
// change.
type SelfTransitionPostBreaksGuardRule struct{}

func (SelfTransitionPostBreaksGuardRule) ID() string { return "self-transition-post-breaks-guard" }

func (SelfTransitionPostBreaksGuardRule) Description() string {
	return "A self-transition whose post does not keep what its guard reads may be two states written as one."
}

func (r SelfTransitionPostBreaksGuardRule) Check(in *Input) []Finding {
	if in.Diagram == nil {
		return nil
	}

	var findings []Finding
	for _, edge := range in.Diagram.Edges {
		if edge.Src != edge.Dst {
			continue
		}
		loose := notKept(edge, in.Diagram.Vars[edge.Src])
		if len(loose) == 0 {
			continue
		}
		findings = append(findings, Finding{
			File:      in.File,
			StartLine: edge.Line,
			EndLine:   edge.Line,
			RuleID:    r.ID(),
			Severity:  SeverityHint,
			Message: fmt.Sprintf(
				"State `%s` has a self-transition on `%s` guarded by `%s` whose post `%s` does not keep %s, which the guard reads, so the guard may be false after the step. Assume the guard holds, apply the post, and evaluate the guard again: if it can be false, `%[1]s` is two states - the one where the guard holds and the one after the post - and the edge should go to a second state. If the guard always holds again, keep what it reads with @eq(v', v) in the post, or leave the loop as it is.",
				edge.Src, edge.Event.Label(), printed(edge.Guard), printed(edge.Post), quotedList(loose),
			),
		})
	}
	return findings
}

// notKept are the names e's guard reads that may differ on the next step,
// in the order the guard reads them: the state variables vars that e's post
// does not keep, and the fields of e's event. Any other name is not the
// diagram's to change, such as a constant.
func notKept(e core.Edge, vars []string) []string {
	mutable := map[string]bool{}
	for _, v := range vars {
		mutable[v] = true
	}
	for _, f := range e.Event.Fields {
		mutable[f.Name] = true
	}
	kept := map[string]bool{}
	for _, c := range clause.Normalize(e.Post).Conjuncts {
		if b, ok := c.(clause.Builtin); ok {
			if v, ok := keeps(b); ok {
				kept[v] = true
			}
		}
	}
	var loose []string
	for _, v := range clause.FreeVars(e.Guard) {
		if mutable[v.Name] && !kept[v.Name] && !slices.Contains(loose, v.Name) {
			loose = append(loose, v.Name)
		}
	}
	return loose
}

// keeps is the variable b says is unchanged by the step, if it says so:
// @eq(v', v), @eq(v, v') or @frame(v', v).
func keeps(b clause.Builtin) (string, bool) {
	switch {
	case b.Op == clause.OpEq, b.Op == clause.OpFrame && len(b.Args) == 2:
	default:
		return "", false
	}
	x, okX := b.Args[0].(clause.Free)
	y, okY := b.Args[1].(clause.Free)
	if !okX || !okY || x.Name != y.Name || x.Primed == y.Primed {
		return "", false
	}
	if b.Op == clause.OpFrame && !x.Primed {
		return "", false
	}
	return x.Name, true
}

func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	return strings.Join(quoted, ", ")
}

// printed is f as the clause syntax writes it. A formula the reader read
// prints; should one not, the finding still names the edge by its line.
func printed(f clause.And) string {
	s, err := clause.Print(f)
	if err != nil {
		return "(unprintable)"
	}
	return s
}
