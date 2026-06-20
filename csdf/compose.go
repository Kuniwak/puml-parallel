package csdf

import (
	"fmt"
)

// Compose returns the single diagram unchanged, or the CSP interface-parallel
// composition of multiple diagrams over the given synchronization events.
func Compose(diagrams []Diagram, sync []Event) (Diagram, error) {
	if len(diagrams) == 1 {
		return diagrams[0], nil
	}

	composite, err := ComposeParallel(diagrams, sync)
	if err != nil {
		return Diagram{}, fmt.Errorf("composing diagrams: %w", err)
	}
	return composite, nil
}
