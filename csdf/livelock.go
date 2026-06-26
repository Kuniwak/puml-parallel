package csdf

import (
	"fmt"
	"sort"
	"strings"
)

// Livelock is a reachable τ-only cycle: a divergence witness. Stem is a path of
// edges from the start state to the cycle entry (it may contain visible events).
// Cycle is the τ-only cycle itself: an ordered edge list whose events are all τ,
// where Cycle[0].Src is the entry state and Cycle[len-1].Dst == Cycle[0].Src.
type Livelock struct {
	Stem  []Edge
	Cycle []Edge
}

// CheckLivelockFree reports whether d is livelock free, i.e. has no τ-only cycle
// reachable from the start state. When a livelock exists it returns a
// deterministic witness and ok == false; otherwise it returns (nil, true).
//
// The analysis is purely structural over event labels: natural-language Guard and
// Post predicates are not evaluated. A diagram with no τ edges is livelock free.
func CheckLivelockFree(d *Diagram) (witness *Livelock, ok bool) {
	// Index all outgoing edges by source state.
	out := make(map[StateID][]Edge)
	for _, e := range d.Edges {
		out[e.Src] = append(out[e.Src], e)
	}

	reachable := reachableStates(d.StartEdge.Dst, out)

	// τ-only successor index restricted to reachable sources, deterministically
	// ordered so the witness is reproducible.
	tauOut := make(map[StateID][]Edge)
	for _, e := range d.Edges {
		if e.Event != Tau {
			continue
		}
		if _, ok := reachable[e.Src]; !ok {
			continue
		}
		tauOut[e.Src] = append(tauOut[e.Src], e)
	}
	for s := range tauOut {
		sortEdges(tauOut[s])
	}

	cycle := findTauCycle(tauOut)
	if cycle == nil {
		return nil, true
	}
	stem := stemTo(d.StartEdge.Dst, cycle[0].Src, out)
	return &Livelock{Stem: stem, Cycle: cycle}, false
}

// RenderLivelock renders a witness as human-readable lines, one transition per
// line as "Src --event--> Dst". The stem (which may carry visible events) is
// printed first and omitted when empty, followed by a "cycle:" header and the
// τ-only cycle.
func RenderLivelock(w *Livelock) string {
	var sb strings.Builder
	for _, e := range w.Stem {
		sb.WriteString(renderEdge(e))
	}
	sb.WriteString("cycle:\n")
	for _, e := range w.Cycle {
		sb.WriteString(renderEdge(e))
	}
	return sb.String()
}

func renderEdge(e Edge) string {
	return fmt.Sprintf("%s --%s--> %s\n", e.Src, e.Event, e.Dst)
}

// reachableStates returns every state reachable from start over all edges.
func reachableStates(start StateID, out map[StateID][]Edge) map[StateID]struct{} {
	reachable := map[StateID]struct{}{start: {}}
	queue := []StateID{start}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, e := range out[s] {
			if _, ok := reachable[e.Dst]; !ok {
				reachable[e.Dst] = struct{}{}
				queue = append(queue, e.Dst)
			}
		}
	}
	return reachable
}

// dfs coloring states.
const (
	white = 0
	grey  = 1
	black = 2
)

// findTauCycle returns the first τ-only cycle found by a deterministic iterative
// DFS over tauOut, or nil if none exists. The cycle is an ordered edge list
// closed on itself (Cycle[0].Src == Cycle[len-1].Dst).
func findTauCycle(tauOut map[StateID][]Edge) []Edge {
	color := make(map[StateID]int)
	for _, root := range sortedStateIDs(tauOut) {
		if color[root] != white {
			continue
		}
		if cycle := dfsTauCycle(root, tauOut, color); cycle != nil {
			return cycle
		}
	}
	return nil
}

type dfsFrame struct {
	node StateID
	idx  int
}

// dfsTauCycle runs an iterative 3-color DFS from root. An edge into a grey node is
// a back-edge that closes a cycle, which is reconstructed from the grey path.
func dfsTauCycle(root StateID, tauOut map[StateID][]Edge, color map[StateID]int) []Edge {
	color[root] = grey
	entryEdge := make(map[StateID]Edge)
	stack := []dfsFrame{{node: root}}
	for len(stack) > 0 {
		i := len(stack) - 1
		node := stack[i].node
		edges := tauOut[node]
		if stack[i].idx >= len(edges) {
			color[node] = black
			stack = stack[:i]
			continue
		}
		e := edges[stack[i].idx]
		stack[i].idx++
		switch color[e.Dst] {
		case white:
			color[e.Dst] = grey
			entryEdge[e.Dst] = e
			stack = append(stack, dfsFrame{node: e.Dst})
		case grey:
			return reconstructCycle(stack, entryEdge, e)
		}
	}
	return nil
}

// reconstructCycle builds the ordered cycle from the grey-path stack. back is the
// back-edge from the current node to the grey node back.Dst already on the stack;
// the cycle is the stack suffix from back.Dst onward, closed by back.
func reconstructCycle(stack []dfsFrame, entryEdge map[StateID]Edge, back Edge) []Edge {
	p := 0
	for i := range stack {
		if stack[i].node == back.Dst {
			p = i
			break
		}
	}
	cycle := make([]Edge, 0, len(stack)-p)
	for i := p + 1; i < len(stack); i++ {
		cycle = append(cycle, entryEdge[stack[i].node])
	}
	return append(cycle, back)
}

// stemTo returns a shortest path of edges from start to target over all edges, or
// nil when target == start. Edges are scanned in sorted order for determinism.
func stemTo(start, target StateID, out map[StateID][]Edge) []Edge {
	if start == target {
		return nil
	}
	prev := make(map[StateID]Edge)
	visited := map[StateID]struct{}{start: {}}
	queue := []StateID{start}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		edges := append([]Edge(nil), out[s]...)
		sortEdges(edges)
		for _, e := range edges {
			if _, ok := visited[e.Dst]; ok {
				continue
			}
			visited[e.Dst] = struct{}{}
			prev[e.Dst] = e
			if e.Dst == target {
				return buildPath(prev, start, target)
			}
			queue = append(queue, e.Dst)
		}
	}
	return nil
}

// buildPath walks predecessor edges from target back to start and reverses them.
func buildPath(prev map[StateID]Edge, start, target StateID) []Edge {
	var path []Edge
	for cur := target; cur != start; {
		e := prev[cur]
		path = append(path, e)
		cur = e.Src
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func sortedStateIDs(m map[StateID][]Edge) []StateID {
	ids := make([]StateID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
