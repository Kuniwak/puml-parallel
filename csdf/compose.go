package csdf

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/core"
)

// Compose returns the single diagram unchanged, or the CSP interface-parallel
// composition of multiple diagrams over the given synchronization events.
func Compose(diagrams []core.Diagram, sync []core.Event) (core.Diagram, error) {
	if len(diagrams) == 1 {
		return diagrams[0], nil
	}

	composite, err := core.ComposeParallel(diagrams, sync)
	if err != nil {
		return core.Diagram{}, fmt.Errorf("composing diagrams: %w", err)
	}
	return composite, nil
}
