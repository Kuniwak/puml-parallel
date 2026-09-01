package tools

import (
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// ParseEvents parses a semicolon-separated list of event names as written on
// the command line. Surrounding whitespace is trimmed and empty entries are
// dropped, so an empty string yields no events.
func ParseEvents(s string) []csdf.Event {
	if s == "" {
		return nil
	}
	var events []csdf.Event
	for _, event := range strings.Split(s, ";") {
		trimmed := strings.TrimSpace(event)
		if trimmed != "" {
			events = append(events, csdf.Event(trimmed))
		}
	}
	return events
}
