package coreio

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// Write writes d in the syntax Read reads, so that Read(Write(d)) is d up to
// α-equivalence of its clauses: Read keeps the states in the order Write
// writes them, which is d's. A state is named by its ID. The syntax holds
// one end edge, so a diagram with more is an error.
func Write(d *core.Diagram) (string, error) {
	return write(d, func(core.Edge) (string, error) { return "", nil })
}

// write is Write, with the line comment gloss gives written above each
// edge it gives one for.
func write(d *core.Diagram, gloss func(core.Edge) (string, error)) (string, error) {
	var b strings.Builder
	if err := writeBody(&b, d, gloss, true); err != nil {
		return "", err
	}
	b.WriteString("@enduml\n")
	return b.String(), nil
}

// writeBody writes d as write does, but for the closing @enduml, so that a
// caller may add lines before it; it writes the start edge if start.
func writeBody(b *strings.Builder, d *core.Diagram, gloss func(core.Edge) (string, error), start bool) error {
	if len(d.Ends) > 1 {
		return fmt.Errorf("coreio.Write: %d end edges; the syntax holds one end edge", len(d.Ends))
	}
	b.WriteString("@startuml" + titled(d.Name) + "\n")
	for _, s := range d.States {
		writeState(b, d, s)
	}
	if start {
		if err := writeStart(b, d.Init, d.InitPost); err != nil {
			return fmt.Errorf("coreio.Write: %w", err)
		}
	}
	for _, e := range d.Edges {
		g, err := gloss(e)
		if err != nil {
			return fmt.Errorf("coreio.Write: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
		if g != "" {
			b.WriteString("' " + g + "\n")
		}
		if err := writeEdge(b, e); err != nil {
			return fmt.Errorf("coreio.Write: %w", err)
		}
	}
	for _, e := range d.Ends {
		guard, err := clause.Print(e.Guard)
		if err != nil {
			return fmt.Errorf("coreio.Write: the end edge from %s: %w", e.Src, err)
		}
		b.WriteString(e.Src + " --> [*]" + labelled(guard) + "\n")
	}
	for _, a := range d.Assumes {
		fmt.Fprintf(b, "note as %s\n  assume %s\nend note\n", a.ID, a.Text)
	}
	writeChannels(b, d.Channels)
	return nil
}

// writeState writes the state s of d: its declaration, labelled with its
// label or its ID, and its variables with their types.
func writeState(b *strings.Builder, d *core.Diagram, s string) {
	label, ok := d.Labels[s]
	if !ok {
		label = s
	}
	fmt.Fprintf(b, "state %q as %s\n", label, s)
	writeVars(b, "", d, s)
}

// writeVars writes the variables of the state s of d with their types, each
// line after indent.
func writeVars(b *strings.Builder, indent string, d *core.Diagram, s string) {
	for _, v := range d.Vars[s] {
		if t, ok := d.Types[v]; ok {
			fmt.Fprintf(b, "%s%s : %s ; %s\n", indent, s, v, t)
		} else {
			fmt.Fprintf(b, "%s%s : %s\n", indent, s, v)
		}
	}
}

// writeStart writes the start edge to init with post.
func writeStart(b *strings.Builder, init string, post clause.And) error {
	text, err := clause.Print(post)
	if err != nil {
		return fmt.Errorf("the start edge: %w", err)
	}
	b.WriteString("[*] --> " + init + labelled(text) + "\n")
	return nil
}

// writeEdge writes the edge e as src --> dst : event ; guard ; post.
func writeEdge(b *strings.Builder, e core.Edge) error {
	guard, err := clause.Print(e.Guard)
	if err != nil {
		return fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
	}
	post, err := clause.Print(e.Post)
	if err != nil {
		return fmt.Errorf("%s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
	}
	fmt.Fprintf(b, "%s --> %s : %s\n", e.Src, e.Dst, edgeLabel(event(e.Event), guard, post))
	return nil
}

// writeChannels writes the channel note that declares channels, if there
// is any, the channels sorted.
func writeChannels(b *strings.Builder, channels map[string][]types.Type) {
	if len(channels) == 0 {
		return
	}
	b.WriteString("note as channels\n")
	for _, c := range slices.Sorted(maps.Keys(channels)) {
		b.WriteString("  channel " + core.ChannelDecl(c, channels[c]) + "\n")
	}
	b.WriteString("end note\n")
}

// titled is what follows @startuml for a diagram named name.
func titled(name string) string {
	if name == "" {
		return ""
	}
	return " " + name
}

// labelled is the label of an edge that carries only text, if it has any.
func labelled(text string) string {
	if text == "" {
		return ""
	}
	return " : " + text
}

// event writes e as channel{?field|!field}.
func event(e core.Event) string {
	s := e.Channel
	for _, f := range e.Fields {
		if f.Send {
			s += "!" + f.Name
		} else {
			s += "?" + f.Name
		}
	}
	return s
}

// edgeLabel is event ; guard ; post, leaving out what is empty at the end,
// and writing an empty guard before a post as nothing between the two ;.
func edgeLabel(event, guard, post string) string {
	switch {
	case post != "" && guard == "":
		return event + " ; ; " + post
	case post != "":
		return event + " ; " + guard + " ; " + post
	case guard != "":
		return event + " ; " + guard
	}
	return event
}
