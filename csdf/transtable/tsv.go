package transtable

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Notation spells the connectives of a condition. The predicates are natural
// language, so which spelling reads better depends on the reader.
type Notation struct {
	// And joins the conjuncts of a conjunction.
	And string
	// Not comes before a negated formula.
	Not string
	// Exists comes before the variables it binds.
	Exists string
}

var (
	// NotationNatural spells the connectives as words.
	NotationNatural = Notation{And: " and ", Not: "not ", Exists: "exists "}
	// NotationLogical spells the connectives as symbols, the conjunction the
	// way csdf.Conjunction writes it.
	NotationLogical = Notation{And: " ∧ ", Not: "¬", Exists: "∃"}
)

// validate refuses a notation that spells a connective as nothing: a negated
// guard would then read as the guard itself, and two guards as one.
func (n Notation) validate() error {
	if n.And == "" || n.Not == "" || n.Exists == "" {
		return fmt.Errorf("the notation %+v spells a connective as nothing", n)
	}
	return nil
}

// Format says how WriteTSV spells a table.
type Format struct {
	Notation Notation
}

// fixedColumns are the columns of the TSV before the event columns.
var fixedColumns = []string{"state", "name"}

// WriteTSV writes t as a tab-separated table with a header row: the state, its
// name, then a column per event. A cell holds one outcome per line. An event
// spelled like one of the other columns is refused before anything is written,
// since a reader finds a column by its header.
func WriteTSV(w io.Writer, t *Table, f Format) error {
	if err := f.Notation.validate(); err != nil {
		return fmt.Errorf("transtable.WriteTSV: %w", err)
	}

	header := slices.Clone(fixedColumns)
	reserved := append(slices.Clone(fixedColumns), string(Terminated))
	for _, c := range t.Columns {
		if !c.Termination && slices.Contains(reserved, string(c.Event)) {
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

// cell spells the outcomes one per line. The table does not show the states a
// tau path passes, and true guards and postconditions add nothing to a
// condition, so two outcomes may come out the same; a line already written is
// not written again.
func (f Format) cell(os []Outcome) string {
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

// Line spells one outcome as a line of a cell: its condition in brackets, if
// any, then where the diagram goes, or × for a refusal.
func (f Format) Line(o Outcome) string {
	var sb strings.Builder
	if o.Cond != nil {
		sb.WriteString("[" + f.formula(o.Cond) + "] ")
	}
	if o.Refused {
		sb.WriteString("×")
	} else {
		sb.WriteString("→ " + string(o.Dst))
	}
	return sb.String()
}

// formula spells a condition. A predicate is quoted as a JSON string, so that
// no text it holds can be read as a connective, and its arguments follow in
// parentheses. A negated formula other than an atom, and a quantified formula
// among conjuncts, is parenthesised; a quantifier binds to the end of what it
// is in.
func (f Format) formula(fm Formula) string {
	switch fm := fm.(type) {
	case Atom:
		args := make([]string, 0, len(fm.Args))
		for _, v := range fm.Args {
			args = append(args, string(v))
		}
		return quote(fm.Pred) + "(" + strings.Join(args, ", ") + ")"
	case Not:
		if _, ok := fm.Operand.(Atom); ok {
			return f.Notation.Not + f.formula(fm.Operand)
		}
		return f.Notation.Not + "(" + f.formula(fm.Operand) + ")"
	case And:
		cs := make([]string, 0, len(fm.Conjuncts))
		for _, c := range fm.Conjuncts {
			switch c.(type) {
			case And, Exists:
				cs = append(cs, "("+f.formula(c)+")")
			default:
				cs = append(cs, f.formula(c))
			}
		}
		return strings.Join(cs, f.Notation.And)
	case Exists:
		vars := make([]string, 0, len(fm.Vars))
		for _, v := range fm.Vars {
			vars = append(vars, string(v))
		}
		return f.Notation.Exists + strings.Join(vars, " ") + ". " + f.formula(fm.Body)
	}
	panic(fmt.Sprintf("transtable.Format.formula: unknown formula %T", fm))
}

// quote writes p as a JSON string. HTML characters are left as they are: the
// table is no web page.
func quote(p csdf.Predicate) string {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(string(p)); err != nil {
		panic(fmt.Sprintf("transtable.quote: a string always encodes: %v", err))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
