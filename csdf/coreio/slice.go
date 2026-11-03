package coreio

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// WriteSlice writes s as WriteCanonical writes its diagram, but for the
// start edge when the initial state is outside, then, in a CSDF-IGNORE
// region so that PlantUML draws them but no tool reads them, the
// placeholders of the states outside, the start edge to the initial
// state's placeholder if it is outside, and the cut edges, in canonical
// order.
func WriteSlice(s *core.Sliced) (string, error) {
	canonical, err := Canonical(s.Diagram)
	if err != nil {
		return "", fmt.Errorf("coreio.WriteSlice: %w", err)
	}
	cut, err := Canonical(&core.Diagram{Edges: s.Cut})
	if err != nil {
		return "", fmt.Errorf("coreio.WriteSlice: %w", err)
	}
	var b strings.Builder
	noGloss := func(core.Edge) (string, error) { return "", nil }
	if err := writeBody(&b, canonical, noGloss, s.OutsideInit == ""); err != nil {
		return "", fmt.Errorf("coreio.WriteSlice: %w", err)
	}
	if len(s.Cut) > 0 || s.OutsideInit != "" {
		b.WriteString("' CSDF-IGNORE-BEGIN\n")
		for _, p := range s.Outside {
			writeState(&b, &core.Diagram{Labels: map[string]string{p.ID: p.Label}}, p.ID)
		}
		if s.OutsideInit != "" {
			if err := writeStart(&b, s.OutsideInit, s.OutsideInitPost); err != nil {
				return "", fmt.Errorf("coreio.WriteSlice: %w", err)
			}
		}
		for _, e := range cut.Edges {
			if err := writeEdge(&b, e); err != nil {
				return "", fmt.Errorf("coreio.WriteSlice: %w", err)
			}
		}
		b.WriteString("' CSDF-IGNORE-END\n")
	}
	b.WriteString("@enduml\n")
	return b.String(), nil
}
