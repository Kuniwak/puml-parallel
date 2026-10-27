package csdf

import (
	"cmp"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/types"
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

// Tau is the internal (silent) event. An edge whose event is exactly "tau" is a
// τ-transition (docs/SYNTAX.md, docs/REFINEMENT_ALGORITHM.md §8).
const Tau Event = "tau"

type Diagram struct {
	// Name is the optional PlantUML diagram name written on the @startuml line.
	// It carries no meaning, but it is kept as core.Diagram.Name, so that
	// writing a diagram back keeps it.
	Name      string            `json:"name,omitempty"`
	States    map[StateID]State `json:"states"`
	StartEdge StartEdge         `json:"start_edge"`
	Edges     []Edge            `json:"edges"`
	EndEdge   *EndEdge          `json:"end_edge"`
	// Assumptions are what the diagram assumes of its runs as a whole, such as
	// a fairness assumption, written as assume notes. They are never null in
	// JSON, only empty.
	Assumptions []Assumption `json:"assumptions"`
	// Channels are the channels the channel notes declare, in the order
	// they are written.
	Channels []ChannelDecl `json:"channels,omitempty"`
}

// ChannelDecl declares a channel and the type of each of its fields, in
// order; a channel with no field has none.
type ChannelDecl struct {
	Name   string       `json:"name"`
	Fields []types.Type `json:"fields"`
	Line   int          `json:"line"` // 1-based source line of the declaration.
}

// Assumption is one assume note: a property of the runs of the diagram that no
// guard can state, such as "a transaction whose fee keeps being raised is
// eventually confirmed". It is opaque text like a predicate, and the tools
// that compile proof obligations carry it as a hypothesis.
type Assumption struct {
	ID   string    `json:"id"`
	Text Predicate `json:"text"`
	Line int       `json:"line"` // 1-based source line of the assume line.
}

type State struct {
	Name string     `json:"name"`
	Vars []StateVar `json:"vars"`
	Line int        `json:"line"` // 1-based source line of the start edge.
}

func compareState(a, b State) int {
	return a.Line - b.Line
}

type StateWithID struct {
	State
	ID StateID `json:"id"`
}

// compareStateWithID orders states by source line, falling back to the id. The
// fallback is what makes SortedStates deterministic: composition synthesises
// states that never came from a source line, so every such state has
// line 0 and comparing lines alone leaves them all tied.
func compareStateWithID(a, b StateWithID) int {
	if c := compareState(a.State, b.State); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

func SortedStates(m map[StateID]State) []StateWithID {
	ss := make([]StateWithID, 0, len(m))
	for id, s := range m {
		ss = append(ss, StateWithID{
			ID:    id,
			State: s,
		})
	}
	slices.SortFunc(ss, compareStateWithID)
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
