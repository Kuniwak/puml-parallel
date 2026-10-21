package promote

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// syncPlan is one sync directive worked out against the promotions: the states
// the merged edges land on, and the block of each target map in each of them.
type syncPlan struct {
	sync   Sync
	states []csdf.StateID
	// blocks[state][i] is the block of the i-th target in that state.
	blocks map[csdf.StateID][]Promote
}

// planSyncs reads every sync directive against the promotions and reports what
// cannot mean anything. A sync merges the edges its maps contribute to one
// event, so it needs every map promoted, the event present on every side, and a
// state where all of them move.
func (e *expander) planSyncs() {
	for _, s := range e.global.Syncs {
		plan := syncPlan{sync: s, blocks: map[csdf.StateID][]Promote{}}

		if s.Event == string(csdf.Tau) {
			e.errorf(s.Line, "tau cannot be synced; an internal event two families take together is an observable one")
			continue
		}

		ok := true
		var states []csdf.StateID
		type role struct {
			m    csdf.Var
			name string
		}
		named, taken := map[role]bool{}, map[role]bool{}
		for i, t := range s.Targets {
			// Two occurrences of one map are two instances: one instance cannot
			// take two edges at once, and which edge each takes is the local
			// event it names, so neither the ID nor the event may repeat.
			if named[role{t.Map, t.Param}] {
				e.errorf(s.Line, "this sync names %s(%s) twice; two occurrences of one map are two instances, so give them two IDs", t.Map, t.Param)
				ok = false
				continue
			}
			named[role{t.Map, t.Param}] = true
			if taken[role{t.Map, t.LocalEvent(s.Event)}] {
				e.errorf(s.Line, "this sync takes %q of %q twice; give each occurrence of one map its own local event", t.LocalEvent(s.Event), t.Map)
				ok = false
				continue
			}
			taken[role{t.Map, t.LocalEvent(s.Event)}] = true

			in := e.statesOf(t.Map)
			if len(in) == 0 {
				e.errorf(s.Line, "%q is not promoted anywhere", t.Map)
				ok = false
				continue
			}
			edges := edgesOn(e.locals[t.Map], t.LocalEvent(s.Event))
			if len(edges) == 0 {
				e.errorf(s.Line, "%s has no edge on %q", e.paths[t.Map], t.LocalEvent(s.Event))
				ok = false
				continue
			}
			// Every combination of the sides becomes one merged event, so sides
			// that disagree on their arguments make the merged event's arity
			// depend on which combination it is - and a constrain matches on
			// arity.
			if !sameArguments(edges) {
				e.warnf(s.Line, "the edges of %q on %q do not all take the same arguments, so the merged event does not either", t.Map, s.Event)
			}
			if i == 0 {
				states = in
				continue
			}
			states = intersect(states, in)
		}
		if !ok {
			continue
		}
		if len(states) == 0 {
			e.errorf(s.Line, "the maps of this sync never move in the same state, so the event they take together can never happen")
			continue
		}

		plan.states = states
		for _, g := range states {
			for _, t := range s.Targets {
				block, ok := e.blockOf(t.Map, g)
				if !ok {
					panic(fmt.Errorf("promote: %q has no block in %q, which is in the intersection of the sync's states", t.Map, g))
				}
				plan.blocks[g] = append(plan.blocks[g], block)
			}
		}
		e.plans = append(e.plans, plan)

		// The event is the sync's from here on: the maps do not expand it on
		// their own in the states the merged edges land on.
		for _, t := range s.Targets {
			for _, g := range states {
				e.owned[ownership{g, t.Map, t.LocalEvent(s.Event)}] = true
			}
		}
	}
}

// ownership marks an event as belonging to a sync in one state, so the map's own
// expansion leaves it alone there.
type ownership struct {
	state csdf.StateID
	m     csdf.Var
	event string
}

// mergeSyncs emits one edge per combination of the local edges its maps
// contribute: a guard on one side can pick out any state on the other, so the
// product is what the two families can do together.
func (e *expander) mergeSyncs(edges []ExpandedEdge) []ExpandedEdge {
	for _, plan := range e.plans {
		for _, g := range plan.states {
			blocks := plan.blocks[g]

			choices := make([][]csdf.Edge, len(blocks))
			for i, b := range blocks {
				choices[i] = edgesOn(e.locals[b.Map], plan.sync.Targets[i].LocalEvent(plan.sync.Event))
			}

			for _, combination := range product(choices) {
				parts := make([]edgeParts, len(blocks))
				moved := make([]csdf.Var, len(blocks))
				links := chain(plan.sync.Targets)
				for i, edge := range combination {
					parts[i] = e.parts(blocks[i], edge, plan.sync.Targets[i].Param, links[i])
					moved[i] = blocks[i].Map
				}
				// The merged event is the sync's, whatever local event each side
				// took, and its guard says that the instances of one map differ.
				merged := e.compose(g, plan.sync.Event, parts, e.distinct(plan.sync.Targets), moved...)
				merged.Origin = e.syncOrigin(plan, blocks, parts)
				edges = append(edges, merged)
			}
		}
	}
	return edges
}

// chain is the link of each target's move, in the order of the targets. A map
// one edge moves once goes from m to m'; one it moves k times goes through
// m₁ … m₍ₖ₋₁₎, so that the clauses chain the moves instead of saying m' twice.
func chain(targets []MapRef) []link {
	total := map[csdf.Var]int{}
	for _, t := range targets {
		total[t.Map]++
	}
	seen := map[csdf.Var]int{}
	links := make([]link, len(targets))
	for i, t := range targets {
		k := seen[t.Map]
		seen[t.Map]++
		links[i] = link{before: stage(t.Map, k, total[t.Map]), after: stage(t.Map, k+1, total[t.Map])}
	}
	return links
}

// stage names map m after k of its n moves.
func stage(m csdf.Var, k, n int) string {
	switch k {
	case 0:
		return string(m)
	case n:
		return string(m) + "'"
	}
	return string(m) + subscript(k)
}

func subscript(k int) string {
	var sb strings.Builder
	for _, d := range fmt.Sprint(k) {
		sb.WriteRune('₀' + (d - '0'))
	}
	return sb.String()
}

// distinct is a guard clause for every two occurrences of one map: their IDs
// differ, since they are two instances.
func (e *expander) distinct(targets []MapRef) []ExpandedClause {
	var clauses []ExpandedClause
	for i, a := range targets {
		for _, b := range targets[i+1:] {
			if a.Map != b.Map {
				continue
			}
			c := Clause{Map: string(a.Map), ID: a.Param, Other: b.Param}
			clauses = append(clauses, ExpandedClause{Kind: ClauseDistinct, Text: e.templates.clause(clauseDistinct, c), Map: string(a.Map), IDs: []string{a.Param, b.Param}})
		}
	}
	return clauses
}

func (e *expander) syncOrigin(plan syncPlan, blocks []Promote, parts []edgeParts) string {
	sides := make([]string, len(parts))
	for i, part := range parts {
		sides[i] = fmt.Sprintf("(%s 〈%s〉 → 〈%s〉)", e.paths[blocks[i].Map], part.src, part.dst)
	}
	return fmt.Sprintf("sync: %s %s", plan.sync.Event, strings.Join(sides, " × "))
}

// constrain conjoins each constrain guard onto every generated edge that matches
// the event in channel and field directions. A hand-written edge is left alone: it already
// says everything it was meant to.
func (e *expander) constrain(edges []ExpandedEdge) {
	for _, c := range e.global.Constrains {
		matched := 0
		var taken []string
		for i := range edges {
			if edges[i].Origin == "" {
				continue
			}
			event := edges[i].Edge.Event
			if !sameShape(event, c.Event) {
				continue
			}
			matched++
			for _, f := range event.Fields() {
				taken = appendUnique(taken, f.Name)
			}

			for _, clause := range c.Guard.Clauses() {
				edges[i].Guard = append(edges[i].Guard, e.authored(ClauseConstrain, clause, ExpandedClause{Line: c.Line}))
			}
			edges[i].Edge.Guard = conjoin(edges[i].Guard)
		}
		if matched == 0 {
			e.errorf(c.Line, "no expanded edge is on %s with fields of the directions %s", c.Event.Channel(), directions(c.Event))
			continue
		}
		// The directive is one thing, so a name it gets wrong is said once,
		// however many edges it matched.
		for _, f := range c.Event.Fields() {
			if !slices.Contains(taken, f.Name) {
				e.warnf(c.Line, "%s has no field named %q; the guard says nothing about it", c.Event.Channel(), f.Name)
			}
		}
	}
}

// statesOf reports the states a map is promoted in, in source order.
func (e *expander) statesOf(m csdf.Var) []csdf.StateID {
	var states []csdf.StateID
	for _, p := range e.global.Promotes {
		if p.Map == m && !slices.Contains(states, p.In) {
			states = append(states, p.In)
		}
	}
	return states
}

// blockOf returns the block that promotes m in state g, if there is one.
func (e *expander) blockOf(m csdf.Var, g csdf.StateID) (Promote, bool) {
	for _, p := range e.global.Promotes {
		if p.Map == m && p.In == g {
			return p, true
		}
	}
	return Promote{}, false
}

// edgesOn returns the local edges carrying the named event, in source order.
func edgesOn(local *csdf.Diagram, event string) []csdf.Edge {
	var edges []csdf.Edge
	for _, edge := range local.Edges {
		if edge.Event.Channel() == event {
			edges = append(edges, edge)
		}
	}
	return edges
}

// sameArguments reports whether every edge takes the same argument list.
func sameArguments(edges []csdf.Edge) bool {
	first := edges[0].Event.Fields()
	for _, edge := range edges[1:] {
		if !slices.Equal(edge.Event.Fields(), first) {
			return false
		}
	}
	return true
}

// product returns every way of choosing one edge from each side.
func product(choices [][]csdf.Edge) [][]csdf.Edge {
	out := [][]csdf.Edge{{}}
	for _, side := range choices {
		next := make([][]csdf.Edge, 0, len(out)*len(side))
		for _, prefix := range out {
			for _, edge := range side {
				next = append(next, append(append([]csdf.Edge{}, prefix...), edge))
			}
		}
		out = next
	}
	return out
}

func intersect(a, b []csdf.StateID) []csdf.StateID {
	var out []csdf.StateID
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// sameShape reports whether two events are on one channel with fields of the
// same directions, in order: what a constrain matches, whatever the names.
func sameShape(a, b csdf.Event) bool {
	return a.Channel() == b.Channel() && directions(a) == directions(b)
}

// directions writes the directions of an event's fields in order, as "??!".
func directions(e csdf.Event) string {
	var sb strings.Builder
	for _, f := range e.Fields() {
		sb.WriteString(string(f.Dir))
	}
	return sb.String()
}
