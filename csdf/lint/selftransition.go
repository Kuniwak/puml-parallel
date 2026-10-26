package lint

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// SelfTransitionPostBreaksGuardRule reports a transition from a state back to
// itself that has both a guard and a post. Predicates are natural language and
// so opaque: nothing here can decide whether the post falsifies the guard. What
// it can do is point at the one shape where that question matters.
//
// It matters because a self-transition says the system is in the same situation
// after the event as before it. If the post makes the guard false, that is not
// so - the edge is enabled once and then not again - and the two situations are
// two states that were written as one.
type SelfTransitionPostBreaksGuardRule struct{}

func (SelfTransitionPostBreaksGuardRule) ID() string { return "self-transition-post-breaks-guard" }

func (SelfTransitionPostBreaksGuardRule) Description() string {
	return "A self-transition whose post may falsify its own guard is two states written as one."
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
		if len(edge.Guard.Conjuncts) == 0 || len(edge.Post.Conjuncts) == 0 {
			continue
		}
		findings = append(findings, Finding{
			File:      in.File,
			StartLine: edge.Line,
			EndLine:   edge.Line,
			RuleID:    r.ID(),
			Severity:  SeverityHint,
			Message: fmt.Sprintf(
				"State `%s` has a self-transition on `%s` guarded by `%s` whose post is `%s`. Decide whether the post can make the guard false: assume the guard holds, apply the post, and evaluate the guard again in the resulting state. If it can be false, `%[1]s` is not one state but two - the one where `%[3]s` holds and the one after `%[4]s` - and the edge should go to a second state instead of back to `%[1]s`. If the guard still holds afterwards, say so and leave the loop alone.",
				edge.Src, edge.Event.Label(), printed(edge.Guard), printed(edge.Post),
			),
		})
	}
	return findings
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
