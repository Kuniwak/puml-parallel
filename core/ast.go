package core

import (
	"fmt"
	"strings"
)

type ID string
type StateID ID

type EventID ID

const EventIDTau EventID = "τ"

type Var ID

const True = "true"

type Diagram struct {
	States    map[StateID]State
	StartEdge StartEdge
	Edges     []Edge
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
	if d.StartEdge.Post == "" || d.StartEdge.Post == True {
		sb.WriteString(fmt.Sprintf("[*] --> %s\n", d.StartEdge.Dst))
	} else {
		sb.WriteString(fmt.Sprintf("[*] --> %s : %s\n", d.StartEdge.Dst, d.StartEdge.Post))
	}

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
		if edge.Post == "" || edge.Post == True {
			sb.WriteString("\n")
			continue
		}
		if edge.Guard == "" || edge.Guard == True {
			sb.WriteString(fmt.Sprintf(" ; %s\n", edge.Post))
			continue
		}
		sb.WriteString(fmt.Sprintf(" ; %s ; %s\n", edge.Guard, edge.Post))
	}

	sb.WriteString("@enduml\n")
	return sb.String()
}
