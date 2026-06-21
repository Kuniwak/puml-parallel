package csdf

import (
	"sort"
)

// AllEvents returns the sorted set of events used across the given diagrams.
func AllEvents(diagrams []*Diagram) []string {
	set := make(map[Event]struct{})
	for _, diagram := range diagrams {
		for _, edge := range diagram.Edges {
			set[edge.Event] = struct{}{}
		}
	}

	events := make([]string, 0, len(set))
	for event := range set {
		events = append(events, string(event))
	}
	sort.Strings(events)
	return events
}

// CommonEvents returns the sorted events that appear in every diagram.
func CommonEvents(diagrams []*Diagram) []string {
	count := make(map[Event]int)
	for _, diagram := range diagrams {
		seen := make(map[Event]struct{})
		for _, edge := range diagram.Edges {
			seen[edge.Event] = struct{}{}
		}
		for event := range seen {
			count[event]++
		}
	}

	total := len(diagrams)
	common := make([]string, 0)
	for event, c := range count {
		if c == total {
			common = append(common, string(event))
		}
	}
	sort.Strings(common)
	return common
}
