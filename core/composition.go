package core

import (
	"fmt"
)

type StatePair struct {
	Left  State
	Right State
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
		ID:   s.ID(),
		Name: ComposeStateNames(s.Left.Name, s.Right.Name),
		Vars: append(append([]Var{}, s.Left.Vars...), s.Right.Vars...),
	}
}

func ComposeParallel(diagrams []Diagram, syncEvents []EventID) (Diagram, error) {
	if len(diagrams) < 1 {
		return Diagram{}, fmt.Errorf("at least one diagrams are required for parallel composition")
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
				return Diagram{}, err
			}
		}
	}

	return ComposeParallel2(dL, dR, syncEvents)
}

func ComposeParallel2(dL, dR Diagram, syncEvents []EventID) (Diagram, error) {
	ss := make(map[EventID]struct{})
	for _, event := range syncEvents {
		ss[event] = struct{}{}
	}

	initStatePair := StatePair{
		Left:  dL.States[dL.StartEdge.Dst],
		Right: dR.States[dR.StartEdge.Dst],
	}

	out := Diagram{
		States: make(map[StateID]State),
		StartEdge: StartEdge{
			Dst:  initStatePair.ID(),
			Post: ComposePostConditions(dL.StartEdge.Post, dR.StartEdge.Post),
		},
		Edges: make([]Edge, 0),
	}

	marked := make(map[StateID]struct{})
	marked[initStatePair.ID()] = struct{}{}
	queue := []StatePair{initStatePair}
	for len(queue) > 0 {
		if err := composeParallel2(dL, dR, dL.Edges, dR.Edges, &queue, &marked, ss, &out); err != nil {
			return Diagram{}, err
		}
	}
	return out, nil
}

func composeParallel2(dL, dR Diagram, tsL, tsR []Edge, queue *[]StatePair, marked *map[StateID]struct{}, syncEvents map[EventID]struct{}, out *Diagram) error {
	currentPair := (*queue)[0]
	currentPairID := currentPair.ID()
	*queue = (*queue)[1:]

	evs := make(map[EventID]Event)
	evL := make(map[EventID]map[StateID][]Edge)
	evR := make(map[EventID]map[StateID][]Edge)
	for _, tL := range tsL {
		if tL.Src == currentPair.Left.ID {
			evs[tL.Event.ID] = tL.Event
			if _, ok := evL[tL.Event.ID]; !ok {
				evL[tL.Event.ID] = make(map[StateID][]Edge)
			}
			evL[tL.Event.ID][tL.Dst] = append(evL[tL.Event.ID][tL.Dst], tL)
		}
	}

	for _, tR := range tsR {
		if tR.Src == currentPair.Right.ID {
			evs[tR.Event.ID] = tR.Event
			if _, ok := evR[tR.Event.ID]; !ok {
				evR[tR.Event.ID] = make(map[StateID][]Edge)
			}
			evR[tR.Event.ID][tR.Dst] = append(evR[tR.Event.ID][tR.Dst], tR)
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
										Left:  dL.States[dstL],
										Right: dR.States[dstR],
									}
									out.States[nextStatePair.ID()] = nextStatePair.State()
									out.Edges = append(out.Edges, Edge{
										Src:   currentPairID,
										Dst:   nextStatePair.ID(),
										Event: Event{ID: ev},
										Guard: ComposeGuard(eL.Guard, eR.Guard),
										Post:  ComposePostConditions(eL.Post, eR.Post),
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
						Left:  dL.States[dstL],
						Right: currentPair.Right,
					}
					out.States[nextStatePair.ID()] = nextStatePair.State()
					out.Edges = append(out.Edges, Edge{
						Src:   currentPairID,
						Dst:   nextStatePair.ID(),
						Event: evs[ev],
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
						Right: dR.States[dstR],
					}
					out.States[nextStatePair.ID()] = nextStatePair.State()
					out.Edges = append(out.Edges, Edge{
						Src:   currentPairID,
						Dst:   nextStatePair.ID(),
						Event: evs[ev],
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
	return s1 + " || " + s2
}

func ComposeGuard(g1, g2 string) string {
	if g1 == "" || g1 == True {
		return g2
	}
	if g2 == "" || g2 == True {
		return g1
	}
	return g1 + " & " + g2
}

func ComposePostConditions(p1, p2 string) string {
	if p1 == "" || p1 == True {
		return p2
	}
	if p2 == "" || p2 == True {
		return p1
	}
	return p1 + " & " + p2
}
