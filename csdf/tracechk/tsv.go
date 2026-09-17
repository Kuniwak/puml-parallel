// Package tracechk checks whether an event sequence is a trace of a Composable
// State Diagram when every guard is taken as true, and states what has to hold
// of the predicates for it to be a trace in fact.
package tracechk

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// TSVHeader is the only column of a trace TSV.
const TSVHeader = "event"

// ReadTSV reads a trace: a TSV whose header is the single column "event" and
// whose every other row is one event.
func ReadTSV(r io.Reader) ([]csdf.Event, error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.FieldsPerRecord = 1
	cr.LazyQuotes = true

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
