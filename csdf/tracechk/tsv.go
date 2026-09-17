// Package tracechk checks whether an event sequence is a trace of a Composable
// State Diagram when every guard is taken as true, and states what has to hold
// of the predicates for it to be a trace in fact.
package tracechk

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/usererr"
)

// TSVHeader is the only column of a trace TSV.
const TSVHeader = "event"

// ReadError says which trace could not be read. Its own message is the one to
// show: unwrapping to the deepest error would drop the name.
type ReadError struct {
	Name string
	Err  error
}

func (e *ReadError) Error() string { return fmt.Sprintf("%s: %s", e.Name, usererr.Message(e.Err)) }
func (e *ReadError) Unwrap() error { return e.Err }
func (e *ReadError) UserFacing()   {}

// ReadTrace reads the trace TSV in r as the trace named name. A failure is a
// *ReadError carrying the name.
func ReadTrace(name string, r io.Reader) (Trace, error) {
	events, err := ReadTSV(r)
	if err != nil {
		return Trace{}, &ReadError{Name: name, Err: err}
	}
	return Trace{Name: name, Events: events}, nil
}

// ReadTSV reads a trace: a TSV whose header is the single column "event" and
// whose every other row is one event. It is read as CSV with a tab delimiter,
// so a field may be quoted to hold a tab, a newline or a double quote, and an
// unquoted field is taken literally; a second column is an error. Blank lines
// are skipped, as csv.Reader skips them, so an event can never be empty.
func ReadTSV(r io.Reader) ([]csdf.Event, error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.FieldsPerRecord = 1

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("tracechk.ReadTSV: %w", err)
	}
	if len(records) == 0 || records[0][0] != TSVHeader {
		return nil, fmt.Errorf("want a header row %q", TSVHeader)
	}

	events := make([]csdf.Event, 0, len(records)-1)
	for i, record := range records[1:] {
		event := csdf.Event(record[0])
		// A trace is what the environment sees, and it never sees tau.
		if event == csdf.Tau {
			return nil, fmt.Errorf("row %d: %q is internal and cannot appear in a trace", i+2, csdf.Tau)
		}
		events = append(events, event)
	}
	return events, nil
}
