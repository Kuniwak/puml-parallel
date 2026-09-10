package lint

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// SelfTransitionDivergenceRule reports a transition from a state back to
// itself. Such an edge can be taken any number of times in a row, so the
// diagram has the infinite trace prefix -> A -> A -> ... : the event repeats
// without bound and the suffix after A never begins.
//
// A diagram written by an AI has far more of these than one written by hand,
// because a state that should have been split is easier to write as a loop. The
// loop is nonetheless often what the author meant - polling, retrying, adding
// one more item - so this is a HINT: it asks for the trace to be read, and does
// not claim the edge is wrong.
type SelfTransitionDivergenceRule struct{}

func (SelfTransitionDivergenceRule) ID() string { return "self-transition-divergence" }

func (SelfTransitionDivergenceRule) Description() string {
	return "A transition back to its own state can repeat without bound; check that the infinite trace is intended."
}

func (r SelfTransitionDivergenceRule) Check(in *Input) []Finding {
	if in.Diagram == nil {
		return nil
	}

	var findings []Finding
	for _, edge := range in.Diagram.Edges {
		if edge.Src != edge.Dst {
			continue
		}
		findings = append(findings, Finding{
			File:      in.File,
			StartLine: edge.Line,
			EndLine:   edge.Line,
			RuleID:    r.ID(),
			Severity:  SeverityHint,
			Message:   r.message(in.Diagram, edge),
		})
	}
	return findings
}

func (r SelfTransitionDivergenceRule) message(d *csdf.Diagram, edge csdf.Edge) string {
	state := fmt.Sprintf("%q", edge.Src)
	if name := d.States[edge.Src].Name; name != "" && csdf.StateID(name) != edge.Src {
		state = fmt.Sprintf("%q (%s)", edge.Src, name)
	}

	if edge.Event == csdf.Tau {
		return fmt.Sprintf(
			"State %s has a tau self-transition, so the diagram admits the trace prefix -> %[1]s -> %[1]s -> ... in which nothing observable ever happens again: that is divergence, not a loop of work. Check whether this tau was meant to leave %[1]s, and if it was not, prove the diagram livelock-free with csdflivelockfree.",
			state,
		)
	}

	return fmt.Sprintf(
		"State %s has a self-transition on %q, so the diagram admits the trace prefix -> %[1]s -> %[1]s -> ... in which %[2]q repeats without bound and no suffix after %[1]s ever begins. Read that trace against the specification and answer: is every number of repetitions of %[2]q really allowed here, and does %[1]s really look the same after each one? If the repetitions are bounded, or if the system is in a different situation after %[2]q than before it, split %[1]s into the states before and after instead of looping.",
		state, string(edge.Event),
	)
}

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
		if csdf.IsTrue(edge.Guard) || csdf.IsTrue(edge.Post) {
			continue
		}
		findings = append(findings, Finding{
			File:      in.File,
			StartLine: edge.Line,
			EndLine:   edge.Line,
			RuleID:    r.ID(),
			Severity:  SeverityHint,
			Message: fmt.Sprintf(
				"State %q has a self-transition on %q guarded by %q whose post is %q. Decide whether the post can make the guard false: assume the guard holds, apply the post, and evaluate the guard again in the resulting state. If it can be false, %[1]q is not one state but two - the one where %[3]q holds and the one after %[4]q - and the edge should go to a second state instead of back to %[1]q. If the guard still holds afterwards, say so and leave the loop alone.",
				string(edge.Src), string(edge.Event), string(edge.Guard), string(edge.Post),
			),
		})
	}
	return findings
}
