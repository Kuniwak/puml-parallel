package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func diagramOn(channels ...string) *Diagram {
	d := &Diagram{States: []string{"s"}, Init: "s"}
	for _, c := range channels {
		d.Edges = append(d.Edges, Edge{Src: "s", Dst: "s", Event: Event{Channel: c}})
	}
	return d
}

func TestChannels(t *testing.T) {
	type testCase struct {
		Diagrams   []*Diagram
		WantAll    []string
		WantCommon []string
	}

	testCases := map[string]testCase{
		"no diagram": {
			Diagrams:   nil,
			WantAll:    []string{},
			WantCommon: []string{},
		},
		"one diagram, each channel once, sorted": {
			Diagrams:   []*Diagram{diagramOn("b", "a", "b")},
			WantAll:    []string{"a", "b"},
			WantCommon: []string{"a", "b"},
		},
		"two diagrams sharing one channel": {
			Diagrams:   []*Diagram{diagramOn("a", "b"), diagramOn("c", "b", "tau")},
			WantAll:    []string{"a", "b", "c", "tau"},
			WantCommon: []string{"b"},
		},
		"a diagram with no edge has no channel in common": {
			Diagrams:   []*Diagram{diagramOn("a"), diagramOn()},
			WantAll:    []string{"a"},
			WantCommon: []string{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup: testCase.Diagrams

			// Exercise
			all := AllChannels(testCase.Diagrams)
			common := CommonChannels(testCase.Diagrams)

			// Verify
			if diff := cmp.Diff(testCase.WantAll, all); diff != "" {
				t.Errorf("AllChannels (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(testCase.WantCommon, common); diff != "" {
				t.Errorf("CommonChannels (-want +got):\n%s", diff)
			}
		})
	}
}
