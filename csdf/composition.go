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

func ComposeParallel(diagrams []*Diagram, syncEvents []Event) (*Diagram, error) {
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
			dL, err = ComposeParallel2(dL, d, syncEvents)
			if err != nil {
				return nil, fmt.Errorf("csdf.ComposeParallel: %w", err)
			}
		}
	}

	return ComposeParallel2(dL, dR, syncEvents)
}

func ComposeParallel2(dL, dR *Diagram, syncEvents []Event) (*Diagram, error) {
	if dL.EndEdge != nil || dR.EndEdge != nil {
		return nil, fmt.Errorf("csdf.ComposeParallel2: end edges are not supported for interface parallel")
	}

	ss := make(map[Event]struct{})
	for _, event := range syncEvents {
		ss[event] = struct{}{}
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
	}

	marked := make(map[StateID]struct{})
	marked[initStatePair.ID()] = struct{}{}
	queue := []StatePair{initStatePair}
	for len(queue) > 0 {
		if err := composeParallel2(dL, dR, dL.Edges, dR.Edges, &queue, &marked, ss, out); err != nil {
			return nil, fmt.Errorf("csdf.ComposeParallel2: %w", err)
		}
	}
	return out, nil
}

func composeParallel2(dL, dR *Diagram, tsL, tsR []Edge, queue *[]StatePair, marked *map[StateID]struct{}, syncEvents map[Event]struct{}, out *Diagram) error {
	currentPair := (*queue)[0]
	currentPairID := currentPair.ID()
	*queue = (*queue)[1:]

	evs := make(map[Event]struct{})
	evL := make(map[Event]map[StateID][]Edge)
	evR := make(map[Event]map[StateID][]Edge)
	for _, tL := range tsL {
		if tL.Src == currentPair.Left.ID {
			evs[tL.Event] = struct{}{}
			if _, ok := evL[tL.Event]; !ok {
				evL[tL.Event] = make(map[StateID][]Edge)
			}
			evL[tL.Event][tL.Dst] = append(evL[tL.Event][tL.Dst], tL)
		}
	}

	for _, tR := range tsR {
		if tR.Src == currentPair.Right.ID {
			evs[tR.Event] = struct{}{}
			if _, ok := evR[tR.Event]; !ok {
				evR[tR.Event] = make(map[StateID][]Edge)
			}
			evR[tR.Event][tR.Dst] = append(evR[tR.Event][tR.Dst], tR)
		}
	}

	for ev := range evs {
		// Para3
		if _, ok := syncEvents[ev]; ok {
			if dstLs, ok := evL[ev]; ok {
				if dstRs, ok := evR[ev]; ok {
					for dstL, esL := range dstLs {
						for dstR, esR := range dstRs {
							for _, eL := range esL {
								for _, eR := range esR {
									nextStatePair := StatePair{
										Left:  StateWithID{ID: dstL, State: dL.States[dstL]},
										Right: StateWithID{ID: dstR, State: dR.States[dstR]},
									}
									out.States[nextStatePair.ID()] = nextStatePair.State()
									out.Edges = append(out.Edges, Edge{
										Src:   currentPairID,
										Dst:   nextStatePair.ID(),
										Event: ev,
										Guard: Conjunction(eL.Guard, eR.Guard),
										Post:  Conjunction(eL.Post, eR.Post),
									})
									if _, ok := (*marked)[nextStatePair.ID()]; !ok {
										*queue = append(*queue, nextStatePair)
										(*marked)[nextStatePair.ID()] = struct{}{}
									}
								}
							}
						}
					}
				}
			}
			continue
		}

		// Para1
		if dstLs, ok := evL[ev]; ok {
			for dstL, esL := range dstLs {
				for _, eL := range esL {
					nextStatePair := StatePair{
						Left:  StateWithID{ID: dstL, State: dL.States[dstL]},
						Right: currentPair.Right,
					}
					out.States[nextStatePair.ID()] = nextStatePair.State()
					out.Edges = append(out.Edges, Edge{
						Src:   currentPairID,
						Dst:   nextStatePair.ID(),
						Event: ev,
						Guard: eL.Guard,
						Post:  eL.Post,
					})
					if _, ok := (*marked)[nextStatePair.ID()]; !ok {
						*queue = append(*queue, nextStatePair)
						(*marked)[nextStatePair.ID()] = struct{}{}
					}
				}
			}
		}

		// Para2
		if dstRs, ok := evR[ev]; ok {
			for dstR, esR := range dstRs {
				for _, eR := range esR {
					nextStatePair := StatePair{
						Left:  currentPair.Left,
						Right: StateWithID{ID: dstR, State: dR.States[dstR]},
					}
					out.States[nextStatePair.ID()] = nextStatePair.State()
					out.Edges = append(out.Edges, Edge{
						Src:   currentPairID,
						Dst:   nextStatePair.ID(),
						Event: ev,
						Guard: eR.Guard,
						Post:  eR.Post,
					})
					if _, ok := (*marked)[nextStatePair.ID()]; !ok {
						*queue = append(*queue, nextStatePair)
						(*marked)[nextStatePair.ID()] = struct{}{}
					}
				}
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
