package transtable

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Format says how WriteTSV spells a table.
type Format struct {
	Notation logic.Notation
}

// fixedColumns are the columns of the TSV before the event columns.
var fixedColumns = []string{"state", "name"}

// WriteTSV writes t as a tab-separated table with a header row: the state, its
// name, then a column per event. A cell holds one outcome per line. An event
// spelled like one of the other columns is refused before anything is written,
// since a reader finds a column by its header.
func WriteTSV(w io.Writer, t *Table, f Format) error {
	if !f.Notation.Valid() {
		return errors.New("transtable.WriteTSV: the zero notation spells nothing; take one from ParseNotation")
	}

	header := slices.Clone(fixedColumns)
	reserved := append(slices.Clone(fixedColumns), TerminationColumn{}.Header())
	for _, c := range t.Columns {
		if ec, ok := c.(EventColumn); ok && slices.Contains(reserved, ec.Header()) {
			return fmt.Errorf("the event %q is spelled like a column of the table, so its column could not be told apart", ec.Event)
		}
		header = append(header, c.Header())
	}

	cw := csv.NewWriter(w)
	cw.Comma = '\t'
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("transtable.WriteTSV: cannot write the header: %w", err)
	}

	for _, row := range t.Rows {
		record := []string{string(row.State), row.Name}
		for _, cell := range row.Cells {
			record = append(record, strings.Join(f.Lines(cell), "\n"))
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

// Lines spells the outcomes of a cell, one line each. The table does not show
// the states a tau path passes, and true guards and postconditions add nothing
// to a condition, so two outcomes may come out the same; a line already
// spelled is not spelled again.
func (f Format) Lines(os []Outcome) []string {
	lines := make([]string, 0, len(os))
	written := make(map[string]struct{}, len(os))
	for _, o := range os {
		line := f.Line(o)
		if _, ok := written[line]; ok {
			continue
		}
		written[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

// Line spells one outcome as a line of a cell: its condition in brackets,
// unless it is true, then what it comes to.
func (f Format) Line(o Outcome) string {
	if logic.IsTrue(o.Cond) {
		return o.Result.String()
	}
	return "[" + f.Notation.Spell(o.Cond) + "] " + o.Result.String()
}
