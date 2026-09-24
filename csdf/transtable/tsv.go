package transtable

import (
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Notation spells the connectives of a condition and the composition of
// postconditions. The predicates are natural language, so which spelling reads
// better depends on the reader.
type Notation struct {
	// And joins the literals of a condition.
	And string
	// Not comes before a negated guard, which is parenthesised.
	Not string
	// Then joins the postconditions along a path, in order.
	Then string
}

var (
	// NotationNatural spells the connectives as words.
	NotationNatural = Notation{And: " and ", Not: "not ", Then: " then "}
	// NotationLogical spells the connectives as symbols: the conjunction the
	// way csdf.Conjunction writes it, and the composition of relations the way
	// Z does.
	NotationLogical = Notation{And: " ∧ ", Not: "¬", Then: " ⨾ "}
)

// Format says how WriteTSV spells a table.
type Format struct {
	Notation Notation
	// Posts writes the postconditions along the path of an outcome. They do
	// not decide whether an event is refused, so they are left out by default.
	Posts bool
}

// Fixed column names of the TSV, before the event columns.
const (
	stateColumn = "state"
	nameColumn  = "name"
)

// WriteTSV writes t as a tab-separated table with a header row: the state, its
// name, then a column per event. A cell holds one outcome per line. An event
// spelled like one of the other columns is refused before anything is written,
// since a reader finds a column by its header.
func WriteTSV(w io.Writer, t *Table, f Format) error {
	header := []string{stateColumn, nameColumn}
	for _, c := range t.Columns {
		if !c.Termination && slices.Contains([]string{stateColumn, nameColumn, string(Terminated)}, string(c.Event)) {
			return fmt.Errorf("the event %q is spelled like a column of the table, so its column could not be told apart", c.Event)
		}
		header = append(header, c.name())
	}

	cw := csv.NewWriter(w)
	cw.Comma = '\t'
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("transtable.WriteTSV: cannot write the header: %w", err)
	}

	for _, row := range t.Rows {
		record := []string{string(row.State), row.Name}
		for _, cell := range row.Cells {
			record = append(record, f.cell(cell))
		}
		if err := cw.Write(record); err != nil {
			return fmt.Errorf("transtable.WriteTSV: cannot write the row of %s: %w", row.State, err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("transtable.WriteTSV: cannot write: %w", err)
	}
	return nil
}

// cell spells the outcomes one per line. Two tau paths that meet again lead to
// the same outcomes, and whatever the table leaves out - the states passed, and
// the postconditions unless asked for - may be all that tells two outcomes
// apart, so a line already written is not written again.
func (f Format) cell(os []Outcome) string {
	lines := make([]string, 0, len(os))
	for _, o := range os {
		if line := f.outcome(o); !slices.Contains(lines, line) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// name is the header of the column. Termination is spelled the way PlantUML
// spells the end of a diagram.
func (c Column) name() string {
	if c.Termination {
		return string(Terminated)
	}
	return string(c.Event)
}

// outcome spells one outcome: its condition in brackets, if any, then its
// postconditions after a slash when they are asked for and say anything, then
// where the diagram goes, or × for a refusal.
func (f Format) outcome(o Outcome) string {
	var sb strings.Builder
	if len(o.Cond) > 0 {
		sb.WriteString("[" + f.cond(o.Cond) + "] ")
	}
	// A path whose postconditions are all true says nothing about them. One
	// that says something keeps its true ones too: a true postcondition lets
	// the values be anything, so it is not the identity of the composition.
	if f.Posts && slices.ContainsFunc(o.Posts, isNotTrue) {
		sb.WriteString("/ " + f.posts(o.Posts) + " ")
	}
	if o.Refused {
		sb.WriteString("×")
	} else {
		sb.WriteString("→ " + string(o.Dst))
	}
	return sb.String()
}

func (f Format) cond(c Cond) string {
	literals := make([]string, 0, len(c))
	for _, l := range c {
		if l.Negated {
			literals = append(literals, f.Notation.Not+"("+string(l.Pred)+")")
		} else {
			literals = append(literals, string(l.Pred))
		}
	}
	return strings.Join(literals, f.Notation.And)
}

func (f Format) posts(ps []csdf.Predicate) string {
	spelled := make([]string, 0, len(ps))
	for _, p := range ps {
		if csdf.IsTrue(p) {
			p = csdf.PredicateTrue
		}
		spelled = append(spelled, string(p))
	}
	return strings.Join(spelled, f.Notation.Then)
}

func isNotTrue(p csdf.Predicate) bool { return !csdf.IsTrue(p) }
