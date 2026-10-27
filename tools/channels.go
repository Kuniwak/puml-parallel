package tools

import (
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf"
)

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
