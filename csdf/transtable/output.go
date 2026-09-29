package transtable

import (
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Output says what a table is written as. The zero Output names nothing.
type Output int

const (
	// OutputTSV writes a table with WriteTSV, for a reader.
	OutputTSV Output = iota + 1
	// OutputJSON writes it with WriteJSON, for a program.
	OutputJSON
)

// The names of the Outputs.
const (
	OutputNameTSV  = "tsv"
	OutputNameJSON = "json"
)

var outputNamed = map[string]Output{
	OutputNameTSV:  OutputTSV,
	OutputNameJSON: OutputJSON,
}

// ParseOutput returns the Output named name. The names are the core's, so
// that a front end other than the command line names them the same way.
func ParseOutput(name string) (Output, error) {
	o, ok := outputNamed[name]
	if !ok {
		return 0, fmt.Errorf("unknown output %q (want %s or %s)", name, OutputNameTSV, OutputNameJSON)
	}
	return o, nil
}

// SpellsConditions reports whether o spells conditions in a notation. JSON
// gives them as syntax trees, so a notation means nothing to it.
func (o Output) SpellsConditions() bool { return o == OutputTSV }

// Write writes t as o says. n spells the conditions when o spells them, and
// is not read otherwise.
func Write(w io.Writer, t *Table, o Output, n logic.Notation) error {
	switch o {
	case OutputTSV:
		return WriteTSV(w, t, Format{Notation: n})
	case OutputJSON:
		return WriteJSON(w, t)
	}
	return fmt.Errorf("transtable.Write: %w", fmt.Errorf("unknown output %d; take one from ParseOutput", o))
}
