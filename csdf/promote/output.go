package promote

import (
	"encoding/json"
	"fmt"
	"io"
)

// Output says what an expansion is written as. The zero Output names nothing.
type Output int

const (
	// OutputPUML writes the expanded diagram as PlantUML, which every other
	// tool reads.
	OutputPUML Output = iota + 1
	// OutputJSON writes the Expansion itself - the diagram and every edge with
	// its origin and its clauses - for a program that must not parse the
	// wording back.
	OutputJSON
)

// The names of the Outputs.
const (
	OutputNamePUML = "puml"
	OutputNameJSON = "json"
)

var outputNamed = map[string]Output{
	OutputNamePUML: OutputPUML,
	OutputNameJSON: OutputJSON,
}

// ParseOutput returns the Output named name. The names are the core's, so that
// a front end other than the command line names them the same way.
func ParseOutput(name string) (Output, error) {
	o, ok := outputNamed[name]
	if !ok {
		return 0, fmt.Errorf("unknown output %q (want %s or %s)", name, OutputNamePUML, OutputNameJSON)
	}
	return o, nil
}

// Write writes x as o says. comments says whether the origin of each generated
// edge goes along: as a line comment above the edge in PlantUML, as the origin
// field in JSON.
func Write(w io.Writer, x *Expansion, o Output, comments bool) error {
	switch o {
	case OutputPUML:
		if comments {
			_, err := io.WriteString(w, x.String())
			return err
		}
		_, err := io.WriteString(w, x.StringWithoutComments())
		return err
	case OutputJSON:
		return writeJSON(w, x, comments)
	}
	return fmt.Errorf("promote.Write: %w", fmt.Errorf("unknown output %d; take one from ParseOutput", o))
}

func writeJSON(w io.Writer, x *Expansion, comments bool) error {
	out := x
	if !comments {
		edges := make([]ExpandedEdge, len(x.Edges))
		for i, e := range x.Edges {
			e.Origin = ""
			edges[i] = e
		}
		out = &Expansion{Diagram: x.Diagram, Edges: edges, Promotes: x.Promotes, Locals: x.Locals}
	}
	if err := json.NewEncoder(w).Encode(out); err != nil {
		return fmt.Errorf("promote.Write: writing JSON: %w", err)
	}
	return nil
}
