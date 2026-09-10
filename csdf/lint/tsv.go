package lint

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

// TSVHeader names the columns WriteTSV writes.
var TSVHeader = []string{"file", "start_line", "end_line", "rule", "severity", "message"}

// WriteTSV writes the findings as a tab-separated table with a header row. The
// header is there because a finding is read as a prompt as often as it is read
// by a script, and a column of bare numbers says nothing on its own.
func WriteTSV(w io.Writer, findings []Finding) error {
	cw := csv.NewWriter(w)
	cw.Comma = '\t'

	if err := cw.Write(TSVHeader); err != nil {
		return fmt.Errorf("lint.WriteTSV: cannot write the header: %w", err)
	}
	for _, f := range findings {
		record := []string{
			f.File,
			strconv.Itoa(f.StartLine),
			strconv.Itoa(f.EndLine),
			f.RuleID,
			string(f.Severity),
			f.Message,
		}
		if err := cw.Write(record); err != nil {
			return fmt.Errorf("lint.WriteTSV: cannot write a finding: %w", err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("lint.WriteTSV: cannot write: %w", err)
	}
	return nil
}
