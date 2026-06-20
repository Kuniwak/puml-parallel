package csdfparsecmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

func Run(stdin io.Reader, stdout, stderr io.Writer) int {
	// Read from standard input
	input, err := io.ReadAll(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error reading from stdin: %v\n", err)
		return 1
	}

	source, err := pngsrc.Extract(input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error reading PlantUML source: %v\n", err)
		return 1
	}

	// Parse with parser
	parser := core.NewParser(source)
	diagram, err := parser.Parse()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Parse error: %v\n", err)
		return 1
	}

	if err := json.NewEncoder(stdout).Encode(diagram); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error writing JSON: %v\n", err)
		return 1
	}

	return 0
}
