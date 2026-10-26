package lint

import (
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
)

// Input is one file, as the rules read it. The source is read once here
// rather than once per rule, so every rule sees the same diagram and the same
// failure.
//
// Diagram is nil exactly when ParseErr is not: a rule that reads transitions
// has nothing to say about a file that does not read, and says so by
// returning nothing.
type Input struct {
	File     string
	Diagram  *core.Diagram
	ParseErr error
}

// NewInput reads .puml text or .png bytes (the embedded PlantUML source is
// extracted from PNG inputs) as one lint input, its clauses as formulas
// (coreio.ReadBytes). The file name is what the findings point at; pass "-"
// for standard input.
func NewInput(file string, content []byte) *Input {
	d, err := coreio.ReadBytes(content)
	if err != nil {
		return &Input{File: file, ParseErr: err}
	}
	return &Input{File: file, Diagram: d}
}
