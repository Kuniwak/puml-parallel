package animation

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
)

// ReadDiagram reads the diagram a session explores: diagramJSON, when it
// is given (csdfparse's or csdfpromote -format json's output, which holds
// the families of an expansion), and otherwise source, a .puml text or a
// .png, as coreio.ReadBytes reads it. A clause of the old syntax is refused
// with coreio's error.
func ReadDiagram(source []byte, diagramJSON []byte) (*core.Diagram, error) {
	if diagramJSON != nil {
		d, err := core.UnmarshalDiagram(diagramJSON)
		if err != nil {
			return nil, fmt.Errorf("animation.ReadDiagram: %w", err)
		}
		return d, nil
	}
	d, err := coreio.ReadBytes(source)
	if err != nil {
		return nil, fmt.Errorf("animation.ReadDiagram: %w", err)
	}
	return d, nil
}
