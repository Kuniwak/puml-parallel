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
	States map[StateID]State
	Edges  []Edge
}

type State struct {
	ID   StateID
	Name string
	Vars []Var
}

type Edge struct {
	Src   StateIDOrStartOrEnd
	Dst   StateIDOrStartOrEnd
	Event Event
	Guard string
	Post  string
}

// StateIDOrStartOrEnd は IsStartOrEnd が真なら初期状態または終了状態、それ以外の場合は ID の指す StateID を表す。
type StateIDOrStartOrEnd struct {
	ID           StateID
	IsStartOrEnd bool
}

type Event struct {
	ID     EventID
	Params []Var
}

func (s StateIDOrStartOrEnd) String() string {
	if s.IsStartOrEnd {
		return "[*]"
	}
	return string(s.ID)
}

func (s StateIDOrStartOrEnd) IsState(stateID StateID) bool {
	return !s.IsStartOrEnd && s.ID == stateID
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
		srcStr := edge.Src.String()
		dstStr := edge.Dst.String()
		sb.WriteString(fmt.Sprintf("%s --> %s : %s", srcStr, dstStr, edge.Event.ID))
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
