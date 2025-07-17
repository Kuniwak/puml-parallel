package main

import (
	"fmt"
	"strings"
)

type ID string
type StateID ID
type EventID ID
type Var ID

type Diagram struct {
	States map[StateID]State
	Edges  []Edge
}

type State struct {
	ID   StateID
	Name string
	Vars []Var
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
	
	sb.WriteString("@enduml\n")
	return sb.String()
}