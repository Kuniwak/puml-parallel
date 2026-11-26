package animation

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/v5/csdf/core"
	"github.com/Kuniwak/puml-parallel/v5/csdf/coreio"
)

// ReadDiagram reads the diagram a session explores, source, a .puml text
// or a .png, as coreio.ReadExpandedBytes reads it: a promotion is explored as its expansion, families included,
// each !include resolved by include reading against base, and the warnings
// of the promotion are returned. A clause of the old syntax is refused with
// coreio's error.
func ReadDiagram(source []byte, base string, include coreio.Include) (*core.Diagram, []core.Warning, error) {
	d, warnings, err := coreio.ReadExpandedBytes(source, base, include)
	if err != nil {
		return nil, nil, fmt.Errorf("animation.ReadDiagram: %w", err)
	}
	return d, warnings, nil
}
