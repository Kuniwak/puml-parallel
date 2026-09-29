package transtable

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// jsonTable is the JSON encoding of a Table. Every list is a list, never null,
// so that a reader need not tell an empty one from a missing one.
type jsonTable struct {
	Columns     []jsonColumn   `json:"columns"`
	Rows        []jsonRow      `json:"rows"`
	Unreachable []csdf.StateID `json:"unreachable"`
}

// jsonColumn is a column: Kind is "event", with Event, or "termination".
type jsonColumn struct {
	Kind  string     `json:"kind"`
	Event csdf.Event `json:"event,omitempty"`
}

type jsonRow struct {
	State csdf.StateID    `json:"state"`
	Name  string          `json:"name"`
	Cells [][]jsonOutcome `json:"cells"`
}

// jsonOutcome is an outcome: Result is "goto", with State, "terminate" or
// "refuse".
type jsonOutcome struct {
	Result    string       `json:"result"`
	State     csdf.StateID `json:"state,omitempty"`
	Condition logic.Tree   `json:"condition"`
}

// WriteJSON writes t as newline-terminated JSON: its columns, each saying
// whether it is an event or termination, its rows, each with a cell per column
// in the same order, and its unreachable states. An outcome gives what it
// comes to and its condition as a syntax tree, logic.Tree, so that a reader
// can take a condition apart without parsing a notation. HTML escaping is off:
// the guards and postconditions are natural language, where <, > and & are
// common.
func WriteJSON(w io.Writer, t *Table) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(jsonOf(t)); err != nil {
		return fmt.Errorf("transtable.WriteJSON: cannot write: %w", err)
	}
	return nil
}

func jsonOf(t *Table) jsonTable {
	jt := jsonTable{
		Columns:     make([]jsonColumn, 0, len(t.Columns)),
		Rows:        make([]jsonRow, 0, len(t.Rows)),
		Unreachable: append(make([]csdf.StateID, 0, len(t.Unreachable)), t.Unreachable...),
	}
	for _, c := range t.Columns {
		jt.Columns = append(jt.Columns, jsonColumnOf(c))
	}
	for _, r := range t.Rows {
		cells := make([][]jsonOutcome, 0, len(r.Cells))
		for _, cell := range r.Cells {
			os := make([]jsonOutcome, 0, len(cell))
			for _, o := range cell {
				os = append(os, jsonOutcomeOf(o))
			}
			cells = append(cells, os)
		}
		jt.Rows = append(jt.Rows, jsonRow{State: r.State, Name: r.Name, Cells: cells})
	}
	return jt
}

func jsonColumnOf(c Column) jsonColumn {
	switch c := c.(type) {
	case EventColumn:
		return jsonColumn{Kind: "event", Event: c.Event}
	case TerminationColumn:
		return jsonColumn{Kind: "termination"}
	}
	panic(fmt.Sprintf("transtable.jsonColumnOf: unknown column %T", c))
}

func jsonOutcomeOf(o Outcome) jsonOutcome {
	jo := jsonOutcome{Condition: logic.TreeOf(o.Cond)}
	switch r := o.Result.(type) {
	case Goto:
		jo.Result, jo.State = "goto", r.State
	case Terminate:
		jo.Result = "terminate"
	case Refuse:
		jo.Result = "refuse"
	default:
		panic(fmt.Sprintf("transtable.jsonOutcomeOf: unknown result %T", r))
	}
	return jo
}
