package lint

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// SelfTransitionPostBreaksGuardRule reports a transition from a state back
// to itself whose post may make its own guard false. A self-transition says
// the system is in the same situation after the event as before it; if the
// post falsifies the guard, it is not - the edge is enabled once and then
// not again - and the two situations are two states written as one.
//
// Whether the post falsifies the guard depends on atoms, which are never
// evaluated here. What the skeleton decides is the converse: a post that
// keeps every state variable the guard reads, by @eq(v', v) or by @frame(v', v)
// without keys, keeps the guard, since an atom reads only its holes. Such an
// edge, and one whose guard reads nothing, is not reported; every other
// self-transition is a HINT naming the state variables not kept. A field
// of the event or a constant the guard reads is not the state's, so it
// does not make the guard false again on the same event.
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
		loose := edge.NotKept(in.Diagram.Vars[edge.Src])
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
				edge.Src, edge.Event.Label(), printed(edge.Guard), printed(edge.Post), quoted(loose, ", "),
			),
		})
	}
	return findings
}

// quoted is names each in backquotes, joined by sep.
func quoted(names []string, sep string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "`" + n + "`"
	}
	return strings.Join(q, sep)
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
