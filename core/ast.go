package core

import (
	"fmt"
	"strings"
)

type ID string
type StateID ID
type EventID ID
type Var ID

type Diagram struct {
	States    map[StateID]State
	StartEdge StartEdge
	Edges     []Edge
	EndEdges  []EndEdge
}

type State struct {
	ID   StateID
	Name string
	Vars []Var
}

type StartEdge struct {
	Dst  StateID
	Post string
}

type Edge struct {
	Src   StateID
	Dst   StateID
	Event Event
	Guard string
	Post  string
}

type EndEdge struct {
	Src   StateID
	Event Event
	Guard string
}

type Event struct {
	ID     EventID
	Params []Var
}

func (d *Diagram) String() string {
	var sb strings.Builder
	sb.WriteString("@startuml\n")

	for _, state := range d.States {
		sb.WriteString(fmt.Sprintf("state \"%s\" as %s\n", state.Name, state.ID))
		for _, v := range state.Vars {
			sb.WriteString(fmt.Sprintf("%s: %s\n", state.ID, v))
		}
	}
	
	// StartEdge
	sb.WriteString(fmt.Sprintf("[*] --> %s : %s\n", d.StartEdge.Dst, d.StartEdge.Post))

	// Regular edges
	for _, edge := range d.Edges {
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
	
	// EndEdges
	for _, endEdge := range d.EndEdges {
		sb.WriteString(fmt.Sprintf("%s --> [*] : %s", endEdge.Src, endEdge.Event.ID))
		if len(endEdge.Event.Params) > 0 {
			sb.WriteString("(")
			for i, param := range endEdge.Event.Params {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(string(param))
			}
			sb.WriteString(")")
		}
		sb.WriteString(fmt.Sprintf(" ; %s\n", endEdge.Guard))
	}

	sb.WriteString("@enduml\n")
	return sb.String()
}
