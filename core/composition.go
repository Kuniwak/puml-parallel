package core

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

func ComplementEdges(es []Edge, ees []EndEdge) []Edge {
	res := make([]Edge, 0, len(es)+len(ees))
	for _, edge := range es {
		res = append(res, edge)
	}
	for _, endEdge := range ees {
		res = append(res, Edge{
			Src:   endEdge.Src,
			Dst:   StateIDOmega,
			Event: EventTick,
			Guard: True,
			Post:  True,
		})
	}
	return res
}

func ComposeParallel2(dL, dR Diagram, syncEvents []EventID) (Diagram, error) {
	initState1 := dL.States[dL.StartEdge.Dst]
	initState2 := dR.States[dR.StartEdge.Dst]

	tsL := ComplementEdges(dL.Edges, dL.EndEdges)
	tsR := ComplementEdges(dR.Edges, dR.EndEdges)

	ss := make(map[EventID]struct{})
	for _, event := range syncEvents {
		ss[event] = struct{}{}
	}

	initStatePair := StatePair{
		Left:  initState1,
		Right: initState2,
	}

	out := Diagram{
		States: make(map[StateID]State),
		StartEdge: StartEdge{
			Dst:  initStatePair.ID(),
			Post: ComposePostConditions(dL.StartEdge.Post, dR.StartEdge.Post),
		},
		Edges:    make([]Edge, 0),
		EndEdges: make([]EndEdge, 0),
	}

	visited := make(map[StateID]struct{})
	queue := []StatePair{initStatePair}
	for len(queue) > 0 {
		if err := composeParallel2(dL, dR, tsL, tsR, &queue, &visited, ss, &out); err != nil {
			return Diagram{}, err
		}
	}
	return out, nil
}

func composeParallel2(dL, dR Diagram, tsL, tsR []Edge, queue *[]StatePair, visited *map[StateID]struct{}, syncEvents map[EventID]struct{}, out *Diagram) error {
	if len(*queue) == 0 {
		return nil
	}

	currentPair := (*queue)[0]
	currentPairID := currentPair.ID()
	*queue = (*queue)[1:]
	if _, ok := (*visited)[currentPairID]; ok {
		panic("already visited: " + currentPairID)
	}
	(*visited)[currentPairID] = struct{}{}

	// Para6
	if currentPair.Left.ID == StateIDOmega && currentPair.Right.ID == StateIDOmega {
		out.EndEdges = append(out.EndEdges, EndEdge{
			Src:   currentPairID,
			Event: EventTick,
		})
		return nil
	}

	evs := make(map[EventID]Event)
	evL := make(map[EventID]map[StateID][]Edge)
	evR := make(map[EventID]map[StateID][]Edge)
	for _, tL := range tsL {
		if tL.Src == currentPair.Left.ID {
			evs[tL.Event.ID] = tL.Event
			if _, ok := evL[tL.Event.ID]; !ok {
				evL[tL.Event.ID] = make(map[StateID][]Edge)
			}
			evL[tL.Event.ID][tL.Src] = append(evL[tL.Event.ID][tL.Src], tL)
		}
	}

	for _, tR := range tsR {
		if tR.Src == currentPair.Right.ID {
			evs[tR.Event.ID] = tR.Event
			if _, ok := evR[tR.Event.ID]; !ok {
				evR[tR.Event.ID] = make(map[StateID][]Edge)
			}
			evR[tR.Event.ID][tR.Src] = append(evR[tR.Event.ID][tR.Src], tR)
		}
	}

	for ev := range evs {
		if ev == EventIDTick {
			// Para4
			if _, ok := evL[EventIDTick]; ok {
				nextStatePair := StatePair{
					Left:  StateOmega,
					Right: currentPair.Right,
				}
				if _, ok := (*visited)[nextStatePair.ID()]; !ok {
					out.States[nextStatePair.ID()] = nextStatePair.State()
					out.Edges = append(out.Edges, Edge{
						Src:   currentPairID,
						Dst:   nextStatePair.ID(),
						Event: EventTau,
						Guard: True,
						Post:  True,
					})
					*queue = append(*queue, nextStatePair)
				}
			}

			// Para5
			if _, ok := evR[EventIDTick]; ok {
				nextStatePair := StatePair{
					Left:  currentPair.Left,
					Right: StateOmega,
				}
				if _, ok := (*visited)[nextStatePair.ID()]; !ok {
					out.States[nextStatePair.ID()] = nextStatePair.State()
					out.Edges = append(out.Edges, Edge{
						Src:   currentPairID,
						Dst:   nextStatePair.ID(),
						Event: EventTau,
						Guard: True,
						Post:  True,
					})
					*queue = append(*queue, nextStatePair)
				}
			}
		}

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
									if _, ok := (*visited)[nextStatePair.ID()]; !ok {
										out.Edges = append(out.Edges, Edge{
											Src:   currentPairID,
											Dst:   nextStatePair.ID(),
											Event: Event{ID: ev},
											Guard: ComposeGuard(eL.Guard, eR.Guard),
											Post:  ComposePostConditions(eL.Post, eR.Post),
										})
										*queue = append(*queue, nextStatePair)
									}
								}
							}
						}
					}
				}
			} else {
				continue
			}
		} else {
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
					if _, ok := (*visited)[nextStatePair.ID()]; !ok {
						out.Edges = append(out.Edges, Edge{
							Src:   currentPairID,
							Dst:   nextStatePair.ID(),
							Event: evs[ev],
							Guard: eL.Guard,
							Post:  eL.Post,
						})
						out.States[nextStatePair.ID()] = nextStatePair.State()
						*queue = append(*queue, nextStatePair)
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
					if _, ok := (*visited)[nextStatePair.ID()]; !ok {
						out.Edges = append(out.Edges, Edge{
							Src:   currentPairID,
							Dst:   nextStatePair.ID(),
							Event: evs[ev],
							Guard: eR.Guard,
							Post:  eR.Post,
						})
						out.States[nextStatePair.ID()] = nextStatePair.State()
						*queue = append(*queue, nextStatePair)
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
