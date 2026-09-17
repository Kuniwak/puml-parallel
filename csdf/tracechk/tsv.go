// Package tracechk checks whether an event sequence is a trace of a Composable
// State Diagram when every guard is taken as true, and states what has to hold
// of the predicates for it to be a trace in fact.
package tracechk

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// TSVHeader is the only column of a trace TSV.
const TSVHeader = "event"

// ReadTSV reads a trace: a TSV whose header is the single column "event" and
// whose every other row is one event. Events are free-form text, so a row is
// taken literally: quotes are ordinary characters, and a tab, which would start
// a second column, is an error. A trailing newline is optional.
func ReadTSV(r io.Reader) ([]csdf.Event, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("tracechk.ReadTSV: %w", err)
	}
	rows := strings.Split(strings.TrimSuffix(string(bs), "\n"), "\n")
	for i := range rows {
		rows[i] = strings.TrimSuffix(rows[i], "\r")
	}
	if rows[0] != TSVHeader {
		return nil, fmt.Errorf("want a header row %q", TSVHeader)
	}

	events := make([]csdf.Event, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if strings.Contains(row, "\t") {
			return nil, fmt.Errorf("row %d: want one column, got a tab", i+2)
		}
		event := csdf.Event(row)
		// A trace is what the environment sees, and it never sees tau.
		if event == csdf.Tau {
			return nil, fmt.Errorf("row %d: %q is internal and cannot appear in a trace", i+2, csdf.Tau)
		}
		events = append(events, event)
	}
	return events, nil
}
