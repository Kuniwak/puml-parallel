package coreio

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// Canonical is d with its states ordered by ID and its edges by source,
// event, destination, guard and post as Write writes them: the order the
// tools before csdf/core printed in, so that what a tool prints depends on
// the diagram and not on how it was built. Order means nothing to a
// diagram, so this changes only how it reads.
func Canonical(d *core.Diagram) (*core.Diagram, error) {
	type keyed struct {
		e   core.Edge
		key [5]string
	}
	edges := make([]keyed, len(d.Edges))
	for i, e := range d.Edges {
		guard, err := clause.Print(e.Guard)
		if err != nil {
			return nil, fmt.Errorf("coreio.Canonical: %w", err)
		}
		post, err := clause.Print(e.Post)
		if err != nil {
			return nil, fmt.Errorf("coreio.Canonical: %w", err)
		}
		edges[i] = keyed{e, [5]string{e.Src, event(e.Event), e.Dst, guard, post}}
	}
	slices.SortStableFunc(edges, func(a, b keyed) int {
		for i := range a.key {
			if c := cmp.Compare(a.key[i], b.key[i]); c != 0 {
				return c
			}
		}
		return 0
	})
	out := *d
	out.States = slices.Sorted(slices.Values(d.States))
	out.Edges = make([]core.Edge, len(edges))
	for i, k := range edges {
		out.Edges[i] = k.e
	}
	return &out, nil
}
