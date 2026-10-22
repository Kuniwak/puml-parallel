package coreio

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// Write writes d in the syntax Read reads, so that Read(Write(d)) is d up to
// α-equivalence of its clauses: Read keeps the states in the order Write
// writes them, which is d's. A state is named by its ID. The syntax holds
// one end edge, so a diagram with more is an error.
func Write(d *core.Diagram) (string, error) {
	if len(d.Ends) > 1 {
		return "", fmt.Errorf("coreio.Write: %d end edges; the syntax holds one end edge", len(d.Ends))
	}
	var b strings.Builder
	b.WriteString("@startuml\n")
	for _, s := range d.States {
		fmt.Fprintf(&b, "state %q as %s\n", s, s)
		for _, v := range d.Vars[s] {
			fmt.Fprintf(&b, "%s : %s\n", s, v)
		}
	}
	post, err := clause.Print(d.InitPost)
	if err != nil {
		return "", fmt.Errorf("coreio.Write: the start edge: %w", err)
	}
	b.WriteString("[*] --> " + d.Init + labelled(post) + "\n")
	for _, e := range d.Edges {
		guard, err := clause.Print(e.Guard)
		if err != nil {
			return "", fmt.Errorf("coreio.Write: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
		post, err := clause.Print(e.Post)
		if err != nil {
			return "", fmt.Errorf("coreio.Write: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Channel, err)
		}
		fmt.Fprintf(&b, "%s --> %s : %s\n", e.Src, e.Dst, edgeLabel(event(e.Event), guard, post))
	}
	for _, e := range d.Ends {
		guard, err := clause.Print(e.Guard)
		if err != nil {
			return "", fmt.Errorf("coreio.Write: the end edge from %s: %w", e.Src, err)
		}
		b.WriteString(e.Src + " --> [*]" + labelled(guard) + "\n")
	}
	b.WriteString("@enduml\n")
	return b.String(), nil
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
