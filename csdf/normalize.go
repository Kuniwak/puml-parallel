package csdf

import (
	"fmt"
	"sort"
	"strings"
)

// tau is the internal (silent) event. An edge whose event is exactly "tau" is a
// τ-transition (docs/SYNTAX.md, docs/REFINEMENT_ALGORITHM.md §8).
const tau Event = "tau"

// Normalize converts a diagram into its normal form via subset construction with
// τ-closure (docs/REFINEMENT_ALGORITHM.md §4). The result is deterministic and
// τ-free: each normal-form state is a set of source states, reachable visible
// events lead to the τ-closure of the union of their destinations, and the
// underlying source-state set is encoded in the state Name (e.g. "{s0, s1}").
//
// Multiple edges merged on the same event keep their natural-language Guard/Post
// predicates as a true-aware disjunction. The empty sink state ∅ (a trace not in
// the diagram) is omitted from the output. End edges are not supported.
func Normalize(d *Diagram) (*Diagram, error) {
	if d.EndEdge != nil {
		return nil, fmt.Errorf("csdf.Normalize: end edges are not supported")
	}

	// Index outgoing edges by source state.
	out := make(map[StateID][]Edge)
	for _, e := range d.Edges {
		out[e.Src] = append(out[e.Src], e)
	}

	// Initial normal-form state: τ-closure of the start state.
	initSet := tauClosure(map[StateID]struct{}{d.StartEdge.Dst: {}}, out)
	initID := normalStateID(initSet)

	result := &Diagram{
		States:    make(map[StateID]State),
		StartEdge: StartEdge{Dst: initID, Post: d.StartEdge.Post},
		Edges:     make([]Edge, 0),
	}
	result.States[initID] = State{ID: initID, Name: normalStateName(initSet)}

	marked := map[StateID]struct{}{initID: {}}
	queue := []map[StateID]struct{}{initSet}

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		uID := normalStateID(u)

		// u is already τ-closed (only closures are enqueued), so its visible
		// outgoing edges are exactly those of its members. Group them by event.
		byEvent := make(map[Event][]Edge)
		for s := range u {
			for _, e := range out[s] {
				if e.Event == tau {
					continue
				}
				byEvent[e.Event] = append(byEvent[e.Event], e)
			}
		}

		for _, ev := range sortedEvents(byEvent) {
			contrib := byEvent[ev]
			dstSet := make(map[StateID]struct{}, len(contrib))
			guards := make([]string, 0, len(contrib))
			posts := make([]string, 0, len(contrib))
			for _, e := range contrib {
				dstSet[e.Dst] = struct{}{}
				guards = append(guards, e.Guard)
				posts = append(posts, e.Post)
			}

			v := tauClosure(dstSet, out)
			if len(v) == 0 {
				continue // empty sink ∅: omitted
			}
			vID := normalStateID(v)
			if _, ok := result.States[vID]; !ok {
				result.States[vID] = State{ID: vID, Name: normalStateName(v)}
			}
			result.Edges = append(result.Edges, Edge{
				Src:   uID,
				Dst:   vID,
				Event: ev,
				Guard: disjoin(guards),
				Post:  disjoin(posts),
			})
			if _, ok := marked[vID]; !ok {
				marked[vID] = struct{}{}
				queue = append(queue, v)
			}
		}
	}

	sortEdges(result.Edges)
	return result, nil
}

// tauClosure returns the set of states reachable from set via zero or more
// τ-transitions (including the states of set themselves).
func tauClosure(set map[StateID]struct{}, out map[StateID][]Edge) map[StateID]struct{} {
	closure := make(map[StateID]struct{}, len(set))
	queue := make([]StateID, 0, len(set))
	for s := range set {
		closure[s] = struct{}{}
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, e := range out[s] {
			if e.Event != tau {
				continue
			}
			if _, ok := closure[e.Dst]; !ok {
				closure[e.Dst] = struct{}{}
				queue = append(queue, e.Dst)
			}
		}
	}
	return closure
}

// disjoin combines predicates with a true-aware logical OR. "true" (or the empty
// default) is absorbing: if any disjunct is true the result is "true". Otherwise
// the distinct disjuncts are sorted and joined with " | " for stable output.
func disjoin(preds []string) string {
	seen := make(map[string]struct{}, len(preds))
	disjuncts := make([]string, 0, len(preds))
	for _, p := range preds {
		if p == "" || p == True {
			return True
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		disjuncts = append(disjuncts, p)
	}
	if len(disjuncts) == 0 {
		return True
	}
	sort.Strings(disjuncts)
	return strings.Join(disjuncts, " ∨ ")
}

// normalStateID is the canonical identifier of a normal-form state: its member
// IDs sorted and joined with "_". Same subset ⇒ same ID.
func normalStateID(set map[StateID]struct{}) StateID {
	if len(set) == 0 {
		return "EMPTY"
	}
	return StateID(strings.Join(sortedMemberStrings(set), "_"))
}

// normalStateName is the human-readable label of a normal-form state, e.g.
// "{s0, s1}", encoding the underlying source-state set.
func normalStateName(set map[StateID]struct{}) string {
	return "{" + strings.Join(sortedMemberStrings(set), ", ") + "}"
}

func sortedMemberStrings(set map[StateID]struct{}) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	return ids
}

func sortedEvents(byEvent map[Event][]Edge) []Event {
	events := make([]Event, 0, len(byEvent))
	for ev := range byEvent {
		events = append(events, ev)
	}
	sort.Slice(events, func(i, j int) bool { return events[i] < events[j] })
	return events
}

func sortEdges(edges []Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Src != edges[j].Src {
			return edges[i].Src < edges[j].Src
		}
		if edges[i].Event != edges[j].Event {
			return edges[i].Event < edges[j].Event
		}
		return edges[i].Dst < edges[j].Dst
	})
}
