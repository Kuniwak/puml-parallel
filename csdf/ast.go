package csdf

import (
	"fmt"
	"slices"
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

type Predicate string

const (
	PredicateTrue Predicate = "true"
)

// IsTrue reports whether a predicate string is the omitted/default value,
// which renders as the literal True rather than an opaque symbol. The capitalised
// "True"/"False" written by an author are ordinary natural-language predicates.
func IsTrue(s Predicate) bool {
	return s == "" || s == PredicateTrue
}

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
	Name string     `json:"name"`
	Vars []StateVar `json:"vars"`
	Line int        `json:"line"` // 1-based source line of the start edge.
}

func CompareState(a, b State) int {
	return a.Line - b.Line
}

type StateWithID struct {
	State
	ID StateID `json:"id"`
}

func CompareStateWithID(a, b StateWithID) int {
	return CompareState(a.State, b.State)
}

func SortedStates(m map[StateID]State) []StateWithID {
	ss := make([]StateWithID, 0, len(m))
	for id, s := range m {
		ss = append(ss, StateWithID{
			ID:    id,
			State: s,
		})
	}
	slices.SortFunc(ss, CompareStateWithID)
	return ss
}

type StartEdge struct {
	Dst  StateID   `json:"dst"`
	Post Predicate `json:"post"`
	Line int       `json:"line"` // 1-based source line of the start edge.
}

type Edge struct {
	Src   StateID   `json:"src"`
	Dst   StateID   `json:"dst"`
	Event Event     `json:"event"`
	Guard Predicate `json:"guard"`
	Post  Predicate `json:"post"`
	Line  int       `json:"line"` // 1-based source line of the transition.
}

type EndEdge struct {
	Src   StateID   `json:"src"`
	Guard Predicate `json:"guard"`
	Line  int       `json:"line"` // 1-based source line of the transition.
}

func (d *Diagram) String() string {
	var sb strings.Builder
	sb.WriteString("@startuml\n")

	ss := SortedStates(d.States)

	for _, state := range ss {
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
	if IsTrue(d.StartEdge.Post) {
		sb.WriteString(fmt.Sprintf("[*] --> %s\n", d.StartEdge.Dst))
	} else {
		sb.WriteString(fmt.Sprintf("[*] --> %s : %s\n", d.StartEdge.Dst, d.StartEdge.Post))
	}

	// Regular edges
	for _, edge := range d.Edges {
		sb.WriteString(fmt.Sprintf("%s --> %s : %s", edge.Src, edge.Dst, edge.Event))
		if IsTrue(edge.Post) {
			sb.WriteString("\n")
			continue
		}
		if IsTrue(edge.Guard) {
			sb.WriteString(fmt.Sprintf(" ; %s\n", edge.Post))
			continue
		}
		sb.WriteString(fmt.Sprintf(" ; %s ; %s\n", edge.Guard, edge.Post))
	}

	if d.EndEdge != nil {
		sb.WriteString(fmt.Sprintf("%s --> [*]", d.EndEdge.Src))
		if !IsTrue(d.EndEdge.Guard) {
			sb.WriteString(fmt.Sprintf(" : %s", d.EndEdge.Guard))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("@enduml\n")
	return sb.String()
}
