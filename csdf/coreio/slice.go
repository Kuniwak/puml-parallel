package coreio

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
)

// WriteSlice writes s as WriteCanonical writes its diagram, then, in a
// CSDF-IGNORE region so that PlantUML draws them but no tool reads them, a
// placeholder for every state outside that a cut edge or the start edge
// reaches, labelled with that state's label and "…", the cut edges drawn
// to or from those placeholders, and the start edge to the initial state's
// placeholder if the initial state is outside. A placeholder's ID is the
// state's ID with a suffix no name of the diagram ends up colliding with.
func WriteSlice(s *core.Sliced) (string, error) {
	text, err := WriteCanonical(s.Diagram)
	if err != nil {
		return "", fmt.Errorf("coreio.WriteSlice: %w", err)
	}
	if len(s.Cut) == 0 && s.Diagram.Init != "" {
		return text, nil
	}
	suffix := freshSuffix(s)
	placeholder := func(state string) string {
		if slices.Contains(s.Diagram.States, state) {
			return state
		}
		return state + suffix
	}
	stubs := &core.Diagram{Labels: map[string]string{}}
	outside := map[string]bool{}
	for _, e := range s.Cut {
		for _, st := range []string{e.Src, e.Dst} {
			if !slices.Contains(s.Diagram.States, st) {
				outside[st] = true
			}
		}
		e.Src, e.Dst = placeholder(e.Src), placeholder(e.Dst)
		stubs.Edges = append(stubs.Edges, e)
	}
	if s.Diagram.Init == "" {
		outside[s.From.Init] = true
	}
	for st := range outside {
		stubs.States = append(stubs.States, placeholder(st))
		stubs.Labels[placeholder(st)] = label(s.From, st) + " …"
	}
	stubs, err = Canonical(stubs)
	if err != nil {
		return "", fmt.Errorf("coreio.WriteSlice: %w", err)
	}
	var b strings.Builder
	b.WriteString(strings.TrimSuffix(text, "@enduml\n"))
	b.WriteString("' CSDF-IGNORE-BEGIN\n")
	for _, st := range stubs.States {
		fmt.Fprintf(&b, "state %q as %s\n", stubs.Labels[st], st)
	}
	if s.Diagram.Init == "" {
		b.WriteString("[*] --> " + placeholder(s.From.Init) + "\n")
	}
	for _, e := range stubs.Edges {
		guard, err := clause.Print(e.Guard)
		if err != nil {
			return "", fmt.Errorf("coreio.WriteSlice: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
		post, err := clause.Print(e.Post)
		if err != nil {
			return "", fmt.Errorf("coreio.WriteSlice: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
		fmt.Fprintf(&b, "%s --> %s : %s\n", e.Src, e.Dst, edgeLabel(event(e.Event), guard, post))
	}
	b.WriteString("' CSDF-IGNORE-END\n@enduml\n")
	return b.String(), nil
}

// label is what d labels state: its label, or its ID.
func label(d *core.Diagram, state string) string {
	if l, ok := d.Labels[state]; ok {
		return l
	}
	return state
}

// freshSuffix is "_cut", repeated until no state of the diagram s was cut
// from, with it appended, is a name the diagram already has: a state, an
// assumption or the channel note.
func freshSuffix(s *core.Sliced) string {
	names := map[string]bool{"channels": true}
	for _, st := range s.From.States {
		names[st] = true
	}
	for _, a := range s.From.Assumes {
		names[a.ID] = true
	}
	suffix := "_cut"
	for slices.ContainsFunc(s.From.States, func(st string) bool { return names[st+suffix] }) {
		suffix += "_cut"
	}
	return suffix
}
