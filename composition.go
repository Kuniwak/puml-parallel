package main

import (
	"fmt"
	"strings"
)

type CompositeStateID string

func NewCompositeStateID(components []StateID) CompositeStateID {
	var parts []string
	for _, comp := range components {
		parts = append(parts, string(comp))
	}
	return CompositeStateID(strings.Join(parts, "_"))
}

func (c CompositeStateID) String() string {
	return string(c)
}

type CompositeState struct {
	ID         CompositeStateID
	Name       string
	Vars       map[Var]string
	Components []StateID
}

type CompositeDiagram struct {
	States map[CompositeStateID]CompositeState
	Edges  []CompositeEdge
}

type CompositeEdge struct {
	Src   CompositeStateID
	Dst   CompositeStateID
	Event Event
	Guard string
	Post  string
}

func ComposeParallel(diagrams []Diagram, syncEvents []EventID) (*CompositeDiagram, error) {
	if len(diagrams) == 0 {
		return nil, fmt.Errorf("no diagrams provided")
	}

	syncEventSet := make(map[EventID]bool)
	for _, event := range syncEvents {
		syncEventSet[event] = true
	}

	result := &CompositeDiagram{
		States: make(map[CompositeStateID]CompositeState),
		Edges:  []CompositeEdge{},
	}

	allStates := generateAllCompositeStates(diagrams)
	for _, state := range allStates {
		result.States[state.ID] = state
	}

	for srcState := range result.States {
		edges := generateEdgesFromState(srcState, diagrams, syncEventSet, result.States)
		result.Edges = append(result.Edges, edges...)
	}

	return result, nil
}

func generateAllCompositeStates(diagrams []Diagram) []CompositeState {
	var result []CompositeState
	var components [][]StateID

	for _, diagram := range diagrams {
		var states []StateID
		for stateID := range diagram.States {
			states = append(states, stateID)
		}
		components = append(components, states)
	}

	combinations := cartesianProduct(components)
	
	for _, combo := range combinations {
		compositeID := NewCompositeStateID(combo)
		
		var nameParts []string
		vars := make(map[Var]string)
		
		for i, stateID := range combo {
			state := diagrams[i].States[stateID]
			nameParts = append(nameParts, state.Name)
			
			for _, v := range state.Vars {
				vars[v] = fmt.Sprintf("diagram%d", i)
			}
		}
		
		result = append(result, CompositeState{
			ID:         compositeID,
			Name:       strings.Join(nameParts, " || "),
			Vars:       vars,
			Components: combo,
		})
	}
	
	return result
}

func generateEdgesFromState(srcState CompositeStateID, diagrams []Diagram, syncEvents map[EventID]bool, states map[CompositeStateID]CompositeState) []CompositeEdge {
	var result []CompositeEdge
	srcComponents := states[srcState].Components
	
	for i, diagram := range diagrams {
		currentStateID := srcComponents[i]
		
		for _, edge := range diagram.Edges {
			if !edge.Src.IsState(currentStateID) {
				continue
			}
			
			if syncEvents[edge.Event.ID] {
				syncEdges := generateSyncEdges(srcState, edge, i, diagrams, syncEvents, states)
				result = append(result, syncEdges...)
			} else {
				asyncEdge := generateAsyncEdge(srcState, edge, i, states)
				result = append(result, asyncEdge)
			}
		}
	}
	
	return result
}

func generateSyncEdges(srcState CompositeStateID, triggerEdge Edge, triggerIndex int, diagrams []Diagram, syncEvents map[EventID]bool, states map[CompositeStateID]CompositeState) []CompositeEdge {
	var result []CompositeEdge
	srcComponents := states[srcState].Components
	
	var syncPartners [][]Edge
	allHavePartners := true
	
	for i, diagram := range diagrams {
		if i == triggerIndex {
			syncPartners = append(syncPartners, []Edge{triggerEdge})
			continue
		}
		
		var partners []Edge
		currentStateID := srcComponents[i]
		
		for _, edge := range diagram.Edges {
			if edge.Src.IsState(currentStateID) && edge.Event.ID == triggerEdge.Event.ID {
				partners = append(partners, edge)
			}
		}
		
		if len(partners) == 0 {
			allHavePartners = false
			break
		}
		
		syncPartners = append(syncPartners, partners)
	}
	
	if !allHavePartners {
		return result
	}
	
	combinations := cartesianProductEdges(syncPartners)
	
	for _, combo := range combinations {
		dstComponents := make([]StateID, len(combo))
		var guards []string
		var posts []string
		
		for i, edge := range combo {
			if !edge.Dst.IsStartOrEnd {
				dstComponents[i] = edge.Dst.ID
			} else {
				dstComponents[i] = srcComponents[i]
			}
			if edge.Guard != "" {
				guards = append(guards, fmt.Sprintf("(%s)", edge.Guard))
			}
			if edge.Post != "" {
				posts = append(posts, edge.Post)
			}
		}
		
		dstState := NewCompositeStateID(dstComponents)
		
		guard := strings.Join(guards, " && ")
		if guard == "" {
			guard = "true"
		}
		
		post := strings.Join(posts, " && ")
		
		result = append(result, CompositeEdge{
			Src:   srcState,
			Dst:   dstState,
			Event: triggerEdge.Event,
			Guard: guard,
			Post:  post,
		})
	}
	
	return result
}

func generateAsyncEdge(srcState CompositeStateID, edge Edge, diagramIndex int, states map[CompositeStateID]CompositeState) CompositeEdge {
	srcComponents := states[srcState].Components
	dstComponents := make([]StateID, len(srcComponents))
	copy(dstComponents, srcComponents)
	
	if !edge.Dst.IsStartOrEnd {
		dstComponents[diagramIndex] = edge.Dst.ID
	}
	
	return CompositeEdge{
		Src:   srcState,
		Dst:   NewCompositeStateID(dstComponents),
		Event: edge.Event,
		Guard: edge.Guard,
		Post:  edge.Post,
	}
}

func cartesianProduct(components [][]StateID) [][]StateID {
	if len(components) == 0 {
		return [][]StateID{}
	}
	
	result := [][]StateID{{}}
	
	for _, component := range components {
		var newResult [][]StateID
		for _, existing := range result {
			for _, item := range component {
				newCombination := make([]StateID, len(existing)+1)
				copy(newCombination, existing)
				newCombination[len(existing)] = item
				newResult = append(newResult, newCombination)
			}
		}
		result = newResult
	}
	
	return result
}

func cartesianProductEdges(edgeGroups [][]Edge) [][]Edge {
	if len(edgeGroups) == 0 {
		return [][]Edge{}
	}
	
	result := [][]Edge{{}}
	
	for _, group := range edgeGroups {
		var newResult [][]Edge
		for _, existing := range result {
			for _, edge := range group {
				newCombination := make([]Edge, len(existing)+1)
				copy(newCombination, existing)
				newCombination[len(existing)] = edge
				newResult = append(newResult, newCombination)
			}
		}
		result = newResult
	}
	
	return result
}

func (cd *CompositeDiagram) String() string {
	var sb strings.Builder
	sb.WriteString("@startuml\n")
	
	for _, state := range cd.States {
		sb.WriteString(fmt.Sprintf("state \"%s\" as %s\n", state.Name, state.ID))
		for v, source := range state.Vars {
			sb.WriteString(fmt.Sprintf("%s: %s (%s)\n", state.ID, v, source))
		}
	}
	
	for _, edge := range cd.Edges {
		sb.WriteString(fmt.Sprintf("%s --> %s : %s", edge.Src, edge.Dst, edge.Event.ID))
		if len(edge.Event.Params) > 0 {
			sb.WriteString("(")
			for i, param := range edge.Event.Params {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(string(param))
			}
			sb.WriteString(")")
		}
		sb.WriteString(fmt.Sprintf(" ; %s ; %s\n", edge.Guard, edge.Post))
	}
	
	sb.WriteString("@enduml\n")
	return sb.String()
}