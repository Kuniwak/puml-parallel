package csdf

import (
	"fmt"
)

type StatePair struct {
	Left  StateWithID
	Right StateWithID
}

type Trans struct {
	Src   StateID
	Dst   StateID
	Event Event
}

func (s StatePair) ID() StateID {
	return ComposeStateIDs(s.Left.ID, s.Right.ID)
}

func (s StatePair) State() State {
	return State{
		Name: ComposeStateNames(s.Left.Name, s.Right.Name),
		Vars: append(append([]StateVar{}, s.Left.Vars...), s.Right.Vars...),
	}
}

func (s StatePair) StateWithID() StateWithID {
	return StateWithID{
		ID:    s.ID(),
		State: s.State(),
	}
}

func ComposeParallel(diagrams []*Diagram, syncChannels []string) (*Diagram, error) {
	if len(diagrams) < 1 {
		return nil, fmt.Errorf("csdf.ComposeParallel: at least one diagrams are required for interface parallel")
	}

	if len(diagrams) == 1 {
		return diagrams[0], nil
	}

	dL := diagrams[0]
	dR := diagrams[1]

	if len(diagrams) > 2 {
		for _, d := range diagrams[2:] {
			var err error
			dL, err = ComposeParallel2(dL, d, syncChannels)
			if err != nil {
				return nil, fmt.Errorf("csdf.ComposeParallel: %w", err)
			}
		}
	}

	return ComposeParallel2(dL, dR, syncChannels)
}

// ComposeParallel2 is the interface parallel of dL and dR, synchronised on
// syncChannels: an edge on one of them is taken by both sides together, with
// the fields of the two events standing for one value each.
func ComposeParallel2(dL, dR *Diagram, syncChannels []string) (*Diagram, error) {
	if dL.EndEdge != nil || dR.EndEdge != nil {
		return nil, fmt.Errorf("csdf.ComposeParallel2: end edges are not supported for interface parallel")
	}

	ss := make(map[string]struct{})
	for _, channel := range syncChannels {
		ss[channel] = struct{}{}
	}

	initStatePair := StatePair{
		Left: StateWithID{
			ID:    dL.StartEdge.Dst,
			State: dL.States[dL.StartEdge.Dst],
		},
		Right: StateWithID{
			ID:    dR.StartEdge.Dst,
			State: dR.States[dR.StartEdge.Dst],
		},
	}

	states := make(map[StateID]State)
	states[initStatePair.ID()] = initStatePair.State()

	out := &Diagram{
		States: states,
		StartEdge: StartEdge{
			Dst:  initStatePair.ID(),
			Post: Conjunction(dL.StartEdge.Post, dR.StartEdge.Post),
		},
		Edges: make([]Edge, 0),
		// What each side assumes of its runs, the composition assumes of its
		// own: an assumption is about the whole run, so it has no state to
		// pair and no event to synchronise.
		Assumptions: append(append([]Assumption{}, dL.Assumptions...), dR.Assumptions...),
	}

	marked := make(map[StateID]struct{})
	marked[initStatePair.ID()] = struct{}{}
	queue := []StatePair{initStatePair}
	for len(queue) > 0 {
		if err := composeParallel2(dL, dR, dL.Edges, dR.Edges, &queue, &marked, ss, out); err != nil {
			return nil, fmt.Errorf("csdf.ComposeParallel2: %w", err)
		}
	}
	// The composite is explored through maps, so the edges are appended in a
	// random order. Ordering them canonically makes the output deterministic.
	SortEdges(out.Edges)
	return out, nil
}

func composeParallel2(dL, dR *Diagram, tsL, tsR []Edge, queue *[]StatePair, marked *map[StateID]struct{}, syncChannels map[string]struct{}, out *Diagram) error {
	currentPair := (*queue)[0]
	currentPairID := currentPair.ID()
	*queue = (*queue)[1:]

	add := func(next StatePair, e Edge) {
		out.States[next.ID()] = next.State()
		e.Src, e.Dst, e.Line = currentPairID, next.ID(), 0
		out.Edges = append(out.Edges, e)
		if _, ok := (*marked)[next.ID()]; !ok {
			*queue = append(*queue, next)
			(*marked)[next.ID()] = struct{}{}
		}
	}
	synced := func(e Edge) bool {
		_, ok := syncChannels[e.Event.Channel()]
		return ok
	}

	for _, eL := range tsL {
		if eL.Src != currentPair.Left.ID {
			continue
		}
		left := StateWithID{ID: eL.Dst, State: dL.States[eL.Dst]}
		if !synced(eL) {
			// Para1
			add(StatePair{Left: left, Right: currentPair.Right}, Edge{Event: eL.Event, Guard: eL.Guard, Post: eL.Post})
			continue
		}
		// Para3
		for _, eR := range tsR {
			if eR.Src != currentPair.Right.ID || eR.Event.Channel() != eL.Event.Channel() {
				continue
			}
			e, err := synchronise(eL, eR)
			if err != nil {
				return err
			}
			add(StatePair{Left: left, Right: StateWithID{ID: eR.Dst, State: dR.States[eR.Dst]}}, e)
		}
	}
	for _, eR := range tsR {
		if eR.Src != currentPair.Right.ID || synced(eR) {
			continue
		}
		// Para2
		add(StatePair{Left: currentPair.Left, Right: StateWithID{ID: eR.Dst, State: dR.States[eR.Dst]}}, Edge{Event: eR.Event, Guard: eR.Guard, Post: eR.Post})
	}
	return nil
}

// synchronise is the edge both sides take together on one channel. Its event
// names each field as the left does; a right clause reads its own name for a
// field through a renaming, so no clause text is rewritten. A field chosen
// internally on either side is chosen on the composite.
func synchronise(eL, eR Edge) (Edge, error) {
	fL, fR := eL.Event.Fields(), eR.Event.Fields()
	if len(fL) != len(fR) {
		return Edge{}, fmt.Errorf("csdf.ComposeParallel2: %s and %s (lines %d, %d) synchronise on %s with %d and %d fields", eL.Event, eR.Event, eL.Line, eR.Line, eL.Event.Channel(), len(fL), len(fR))
	}
	fields := make([]Field, len(fL))
	var renames []Rename
	for i := range fL {
		fields[i] = fL[i]
		if fR[i].Dir == Chosen {
			fields[i].Dir = Chosen
		}
		if fR[i].Name != fL[i].Name {
			renames = append(renames, Rename{From: fR[i].Name, To: fL[i].Name})
		}
	}
	return Edge{
		Event: NewEvent(eL.Event.Channel(), fields...),
		Guard: Conjunction(eL.Guard, renamed(eR.Guard, renames)),
		Post:  Conjunction(eL.Post, renamed(eR.Post, renames)),
	}, nil
}

// renamed is p with each clause read through renames.
func renamed(p Predicate, renames []Rename) Predicate {
	clauses := p.Clauses()
	for i, c := range clauses {
		clauses[i] = RenameClause(c, renames)
	}
	return ConjoinClauses(clauses)
}

func ComposeStateIDs(s1, s2 StateID) StateID {
	return s1 + "_" + s2
}

func ComposeStateNames(s1, s2 string) string {
	return "(" + s1 + ", " + s2 + ")"
}
