package csdf

import (
	"fmt"
	"sort"
	"strings"
)

type ID string
type StateID ID

type Event string

type Var ID

type StateVar struct {
	Name Var    `json:"name"`
	Type string `json:"type,omitempty"`
}

const True = "true"

// Tau is the internal (silent) event. An edge whose event is exactly "tau" is a
// τ-transition (docs/SYNTAX.md, docs/REFINEMENT_ALGORITHM.md §8).
const Tau Event = "tau"

type Diagram struct {
	States    map[StateID]State `json:"states"`
	StartEdge StartEdge         `json:"start_edge"`
	Edges     []Edge            `json:"edges"`
	EndEdge   *EndEdge          `json:"end_edge"`
}

type State struct {
	ID   StateID    `json:"id"`
	Name string     `json:"name"`
	Vars []StateVar `json:"vars"`
}

type StartEdge struct {
	Dst  StateID `json:"dst"`
	Post string  `json:"post"`
	Line int     `json:"-"` // 1-based source line of the start edge.
}

type Edge struct {
	Src   StateID `json:"src"`
	Dst   StateID `json:"dst"`
	Event Event   `json:"event"`
	Guard string  `json:"guard"`
	Post  string  `json:"post"`
	Line  int     `json:"-"` // 1-based source line of the transition.
}

type EndEdge struct {
	Src   StateID `json:"src"`
	Guard string  `json:"guard"`
}

func (d *Diagram) String() string {
	var sb strings.Builder
	sb.WriteString("@startuml\n")

	stateIDs := make([]StateID, 0, len(d.States))
	for id := range d.States {
		stateIDs = append(stateIDs, id)
	}
	sort.Slice(stateIDs, func(i, j int) bool { return stateIDs[i] < stateIDs[j] })

	for _, id := range stateIDs {
		state := d.States[id]
		sb.WriteString(fmt.Sprintf("state \"%s\" as %s\n", state.Name, state.ID))
		for _, v := range state.Vars {
			sb.WriteString(fmt.Sprintf("%s: %s", state.ID, v.Name))
			if v.Type != "" {
				sb.WriteString(fmt.Sprintf(" ; %s", v.Type))
			}
			sb.WriteString("\n")
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
		sb.WriteString(fmt.Sprintf("%s --> %s : %s", edge.Src, edge.Dst, edge.Event))
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

	if d.EndEdge != nil {
		sb.WriteString(fmt.Sprintf("%s --> [*]", d.EndEdge.Src))
		if d.EndEdge.Guard != "" {
			sb.WriteString(fmt.Sprintf(" : %s", d.EndEdge.Guard))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("@enduml\n")
	return sb.String()
}
