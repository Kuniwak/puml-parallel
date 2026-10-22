package core

import "fmt"

// ComposeAll is the parallel of ds, left to right, each pair synchronised on
// sync, so that the composite names each field as the first diagram does;
// after each step the states no edge reaches are dropped (Reachable).
func ComposeAll(ds []*Diagram, sync []string) (*Diagram, error) {
	if len(ds) == 0 {
		return nil, fmt.Errorf("core.ComposeAll: no diagram")
	}
	composite := ds[0]
	for _, d := range ds[1:] {
		var err error
		if composite, err = Compose(composite, d, sync); err != nil {
			return nil, fmt.Errorf("core.ComposeAll: %w", err)
		}
		composite = Reachable(composite)
	}
	return composite, nil
}

// PromoteOnly is Promote(p) restricted to the families whose maps keep names
// (Restrict), with the warnings of both; with keep empty it is Promote(p).
func PromoteOnly(p Promotion, keep []string) (*Diagram, []Warning, error) {
	d, warnings, err := Promote(p)
	if err != nil || len(keep) == 0 {
		return d, warnings, err
	}
	maps := make([]string, len(p.Families))
	for i, f := range p.Families {
		maps[i] = f.Map
	}
	restricted, more, err := Restrict(d, maps, keep)
	if err != nil {
		return nil, nil, fmt.Errorf("core.PromoteOnly: %w", err)
	}
	return restricted, append(warnings, more...), nil
}
