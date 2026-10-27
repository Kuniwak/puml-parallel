package animation

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
)

// ReadDiagram reads the diagram a session explores, source, a .puml text
// or a .png, as coreio.ReadExpandedBytes reads it: a promotion is explored as its expansion, families included,
// each !include read by include, and the warnings of the promotion are
// returned. A clause of the old syntax is refused with coreio's error.
func ReadDiagram(source []byte, include coreio.Include) (*core.Diagram, []core.Warning, error) {
	d, warnings, err := coreio.ReadExpandedBytes(source, include)
	if err != nil {
		return nil, nil, fmt.Errorf("animation.ReadDiagram: %w", err)
	}
	return d, warnings, nil
}
