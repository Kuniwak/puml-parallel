package csdf

import (
	"sort"
)

// AllChannels returns the sorted set of channels used across the given
// diagrams, which is what a composition synchronises on and hiding hides.
func AllChannels(diagrams []*Diagram) []string {
	set := make(map[string]struct{})
	for _, diagram := range diagrams {
		for _, edge := range diagram.Edges {
			set[edge.Event.Channel()] = struct{}{}
		}
	}

	channels := make([]string, 0, len(set))
	for channel := range set {
		channels = append(channels, channel)
	}
	sort.Strings(channels)
	return channels
}

// CommonChannels returns the sorted channels that appear in every diagram.
func CommonChannels(diagrams []*Diagram) []string {
	count := make(map[string]int)
	for _, diagram := range diagrams {
		seen := make(map[string]struct{})
		for _, edge := range diagram.Edges {
			seen[edge.Event.Channel()] = struct{}{}
		}
		for channel := range seen {
			count[channel]++
		}
	}

	total := len(diagrams)
	common := make([]string, 0)
	for channel, c := range count {
		if c == total {
			common = append(common, channel)
		}
	}
	sort.Strings(common)
	return common
}
