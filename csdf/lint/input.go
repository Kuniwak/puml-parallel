package lint

import (
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// Input is one file, as the rules read it. The source is parsed once here
// rather than once per rule, so every rule sees the same diagram and the same
// failure.
//
// Diagram is nil exactly when ParseErr is not: a rule that reads transitions
// has nothing to say about a file that does not parse, and says so by returning
// nothing.
type Input struct {
	File     string
	Text     string
	Diagram  *csdf.Diagram
	ParseErr error
}

// NewInput reads .puml text or .png bytes (the embedded PlantUML source is
// extracted from PNG inputs) as one lint input. The file name is what the
// findings point at; pass "-" for standard input.
func NewInput(file string, content []byte) *Input {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return &Input{File: file, ParseErr: err}
	}

	in := &Input{File: file, Text: source}
	diagram, err := csdf.Parse(source)
	if err != nil {
		in.ParseErr = err
		return in
	}
	in.Diagram = diagram
	return in
}
