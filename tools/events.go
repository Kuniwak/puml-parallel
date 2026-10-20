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

// ParseChannels parses a semicolon-separated list of channels as written on
// the command line.
func ParseChannels(s string) ([]string, error) {
	var channels []string
	for _, part := range strings.Split(s, ";") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		channel, err := csdf.ParseChannel(part)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}
