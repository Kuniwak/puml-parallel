package lint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// SelfTransitionDivergenceRule reports a tau edge on a cycle of tau edges
// that the start reaches: the cycle may run forever while nothing observable
// happens, which is divergence (docs/CLAUSE_DESIGN.md, livelock analyses).
// A tau self-loop is the shortest such cycle, and the rule keeps its name.
//
// Whether a cycle can run forever depends on its guards, and an atom is
// never evaluated here: a guard is set aside only when it is false on its
// face (Contradictory), and an edge so guarded breaks every cycle through it.
// Any other cycle is a WARN, a question that csdflivelockfree settles.
//
// A self-loop on an observable event is not divergence and is not reported.
type SelfTransitionDivergenceRule struct{}

func (SelfTransitionDivergenceRule) ID() string { return "self-transition-divergence" }

func (SelfTransitionDivergenceRule) Description() string {
	return "A reachable cycle of tau edges whose guards are not false on their face may run forever: divergence."
}

func (r SelfTransitionDivergenceRule) Check(in *Input) []Finding {
	if in.Diagram == nil {
		return nil
	}
	d := in.Diagram

	var taus []core.Edge
	for _, e := range d.Edges {
		if e.Event.Channel == core.Tau && !Contradictory(e.Guard) {
			taus = append(taus, e)
		}
	}
	reached := d.ReachableStates()

	var findings []Finding
	for _, e := range taus {
		if !slices.Contains(reached, e.Src) {
			continue
		}
		back, ok := tauPath(taus, e.Dst, e.Src)
		if !ok {
			continue
		}
		cycle := append([]string{e.Src}, back...)
		findings = append(findings, Finding{
			File:      in.File,
			StartLine: e.Line,
			EndLine:   e.Line,
			RuleID:    r.ID(),
			Severity:  SeverityWarn,
			Message: fmt.Sprintf(
				"The tau edge `%s` -> `%s` lies on the cycle of tau edges %s, which the start reaches, and its guard is not false on its face, so the diagram may diverge: the cycle may run forever while nothing observable happens. If it cannot, prove the diagram livelock-free with csdflivelockfree; if it can, write a guard that excludes it, or make one of its edges observable.",
				e.Src, e.Dst, quotedPath(cycle),
			),
		})
	}
	return findings
}

// tauPath is a shortest path of states from from to to over the edges taus,
// both ends included, the edges taken in the order they are written.
func tauPath(taus []core.Edge, from, to string) ([]string, bool) {
	parent := map[string]string{from: ""}
	todo := []string{from}
	for len(todo) > 0 {
		s := todo[0]
		todo = todo[1:]
		if s == to {
			var path []string
			for t := to; t != from; t = parent[t] {
				path = append(path, t)
			}
			path = append(path, from)
			slices.Reverse(path)
			return path, true
		}
		for _, e := range taus {
			if _, seen := parent[e.Dst]; e.Src == s && !seen {
				parent[e.Dst] = s
				todo = append(todo, e.Dst)
			}
		}
	}
	return nil, false
}

func quotedPath(states []string) string {
	quoted := make([]string, len(states))
	for i, s := range states {
		quoted[i] = "`" + s + "`"
	}
	return strings.Join(quoted, " -> ")
}
