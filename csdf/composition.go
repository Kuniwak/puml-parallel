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

	// Left to right, so the composite names each field as the first diagram does.
	composite := diagrams[0]
	for _, d := range diagrams[1:] {
		var err error
		composite, err = ComposeParallel2(composite, d, syncChannels)
		if err != nil {
			return nil, fmt.Errorf("csdf.ComposeParallel: %w", err)
		}
	}
	return composite, nil
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
	if err := checkFieldCounts(ss, dL, dR); err != nil {
		return nil, fmt.Errorf("csdf.ComposeParallel2: %w", err)
	}
	// Every pair of edges a synchronised channel could pair is checked for a
	// capture, whether or not a walk reaches it, as the field counts are.
	for _, eL := range dL.Edges {
		for _, eR := range dR.Edges {
			if _, ok := ss[eL.Event.Channel()]; ok && eL.Event.Channel() == eR.Event.Channel() {
				if _, err := synchronise(eL, eR, rightVars(dR, eR)); err != nil {
					return nil, err
				}
			}
		}
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
			e, err := synchronise(eL, eR, rightVars(dR, eR))
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
//
// A renaming must not capture a name: a left field name that a right clause
// reads as a state variable of its own is refused, since the renamed clause
// would read the field instead. The tools do not read clauses, so a name a
// right clause reads otherwise (a ghost of a checker, or a value a tau
// chooses) cannot be told, and naming those apart is the author's. Renamings are simultaneous, so two fields
// swapped (ch?a?b with ch?b?a) are two renamings, not a capture.
func synchronise(eL, eR Edge, rightVars map[Var]struct{}) (Edge, error) {
	fL, fR := eL.Event.Fields(), eR.Event.Fields()
	fields := make([]Field, len(fL))
	var renames []Rename
	for i := range fL {
		fields[i] = fL[i]
		if fR[i].Dir == Chosen {
			fields[i].Dir = Chosen
		}
		if fR[i].Name == fL[i].Name {
			continue
		}
		if _, ok := rightVars[Var(fL[i].Name)]; ok {
			return Edge{}, fmt.Errorf("csdf.ComposeParallel2: %s synchronises with %s (line %d), whose state variable %s its field %s would be renamed onto; name them apart", eL.Event, eR.Event, eR.Line, fL[i].Name, fR[i].Name)
		}
		renames = append(renames, Rename{From: fR[i].Name, To: fL[i].Name})
	}
	guard, err := renamed(eR.Guard, renames)
	if err != nil {
		return Edge{}, err
	}
	post, err := renamed(eR.Post, renames)
	if err != nil {
		return Edge{}, err
	}
	return Edge{
		Event: NewEvent(eL.Event.Channel(), fields...),
		Guard: Conjunction(eL.Guard, guard),
		Post:  Conjunction(eL.Post, post),
	}, nil
}

// renamed is p with each clause read through renames.
func renamed(p Predicate, renames []Rename) (Predicate, error) {
	clauses := p.Clauses()
	for i, c := range clauses {
		r, err := RenameClause(c, renames)
		if err != nil {
			return "", fmt.Errorf("csdf.ComposeParallel2: %w", err)
		}
		clauses[i] = r
	}
	return ConjoinClauses(clauses), nil
}

// rightVars are the state variables a right edge's clauses read: those of the
// states it leaves and enters.
func rightVars(d *Diagram, e Edge) map[Var]struct{} {
	vars := map[Var]struct{}{}
	for _, id := range []StateID{e.Src, e.Dst} {
		for _, v := range d.States[id].Vars {
			vars[v.Name] = struct{}{}
		}
	}
	return vars
}

// checkFieldCounts refuses a synchronised channel whose edges, on either side,
// do not all have one number of fields: the two sides must agree on every
// value, whether or not a walk reaches the edge.
func checkFieldCounts(sync map[string]struct{}, diagrams ...*Diagram) error {
	first := map[string]Edge{}
	for _, d := range diagrams {
		for _, e := range d.Edges {
			channel := e.Event.Channel()
			if _, ok := sync[channel]; !ok {
				continue
			}
			f, ok := first[channel]
			if !ok {
				first[channel] = e
				continue
			}
			if len(f.Event.Fields()) != len(e.Event.Fields()) {
				return fmt.Errorf("%s (line %d) and %s (line %d) synchronise on %s with %d and %d fields", f.Event, f.Line, e.Event, e.Line, channel, len(f.Event.Fields()), len(e.Event.Fields()))
			}
		}
	}
	return nil
}

func ComposeStateIDs(s1, s2 StateID) StateID {
	return s1 + "_" + s2
}

func ComposeStateNames(s1, s2 string) string {
	return "(" + s1 + ", " + s2 + ")"
}
