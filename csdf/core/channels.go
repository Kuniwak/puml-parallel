package core

import (
	"maps"
	"slices"
)

// AllChannels are the channels some edge of some diagram in ds is on,
// sorted and each once.
func AllChannels(ds []*Diagram) []string {
	set := make(map[string]struct{})
	for _, d := range ds {
		maps.Copy(set, channelsOf(d))
	}
	return sortedKeys(set)
}

// CommonChannels are the channels some edge of every diagram in ds is on,
// sorted and each once.
func CommonChannels(ds []*Diagram) []string {
	count := make(map[string]int)
	for _, d := range ds {
		for c := range channelsOf(d) {
			count[c]++
		}
	}
	common := make(map[string]struct{})
	for c, n := range count {
		if n == len(ds) {
			common[c] = struct{}{}
		}
	}
	return sortedKeys(common)
}

// channelsOf are the channels the edges of d are on.
func channelsOf(d *Diagram) map[string]struct{} {
	set := make(map[string]struct{})
	for _, e := range d.Edges {
		set[e.Event.Channel] = struct{}{}
	}
	return set
}

func sortedKeys(set map[string]struct{}) []string {
	return append([]string{}, slices.Sorted(maps.Keys(set))...)
}
