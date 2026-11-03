package coreio

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// WritePromotion writes the global diagram p as written, without expanding
// it, in a canonical order, so that what it prints depends on the diagram
// and not on how it was authored: the modes by ID, each with its
// controller's variables in their order and then its maps by name, and
// its <<promote>> blocks by map, the !include in the block of the first
// mode the family moves in; the start edge, the hand-written edges as
// Canonical orders them, the sync notes by event, the constrain notes by
// their line, the assume notes and the channels. Blocks and directive notes
// get IDs of their own: <mode>_<map>, sync_<event> and constrain<n>. It
// needs no local diagram: ReadPromotion with no Include gives what it
// writes. ReadPromotion(WritePromotion(p)) expands as p does.
func WritePromotion(p core.Promotion) (string, error) {
	d, err := Canonical(controller(p))
	if err != nil {
		return "", fmt.Errorf("coreio.WritePromotion: %w", err)
	}
	families := slices.SortedFunc(slices.Values(p.Families), func(a, b core.Family) int { return cmp.Compare(a.Map, b.Map) })
	var b strings.Builder
	b.WriteString("@startuml" + titled(p.Name) + "\n")
	included := map[string]bool{}
	for _, mode := range d.States {
		label, ok := d.Labels[mode]
		if !ok {
			label = mode
		}
		fmt.Fprintf(&b, "state %q as %s {\n", label, mode)
		writeVars(&b, "  ", d, mode)
		for _, f := range families {
			if slices.Contains(f.Frozen, mode) {
				continue
			}
			fmt.Fprintf(&b, "  state %q as %s_%s <<promote>>", fmt.Sprintf("%s : (%s : %s) ⇸ %s", f.Map, f.ID, f.Key, f.Val), mode, f.Map)
			if included[f.Map] {
				b.WriteString("\n")
				continue
			}
			included[f.Map] = true
			fmt.Fprintf(&b, " {\n    !include %s\n  }\n", includePath(f.Include))
		}
		b.WriteString("}\n")
	}
	if err := writeStart(&b, d.Init, d.InitPost); err != nil {
		return "", fmt.Errorf("coreio.WritePromotion: %w", err)
	}
	for _, e := range d.Edges {
		if err := writeEdge(&b, e); err != nil {
			return "", fmt.Errorf("coreio.WritePromotion: %w", err)
		}
	}
	syncs := slices.SortedFunc(slices.Values(p.Syncs), func(a, b core.Sync) int { return cmp.Compare(a.Event, b.Event) })
	for _, s := range syncs {
		line, err := syncLine(s)
		if err != nil {
			return "", fmt.Errorf("coreio.WritePromotion: %w", err)
		}
		fmt.Fprintf(&b, "note as sync_%s\n  %s\nend note\n", s.Event, line)
	}
	constrains := make([]string, len(p.Constrains))
	for i, c := range p.Constrains {
		guard, err := clause.Print(c.Guard)
		if err != nil {
			return "", fmt.Errorf("coreio.WritePromotion: constrain %s: %w", event(c.Event), err)
		}
		constrains[i] = "constrain " + event(c.Event) + " ; " + guard
	}
	slices.Sort(constrains)
	for i, c := range constrains {
		fmt.Fprintf(&b, "note as constrain%d\n  %s\nend note\n", i+1, c)
	}
	for _, a := range d.Assumes {
		fmt.Fprintf(&b, "note as %s\n  assume %s\nend note\n", a.ID, a.Text)
	}
	writeChannels(&b, d.Channels)
	b.WriteString("@enduml\n")
	return b.String(), nil
}

// controller is the diagram of p's modes as states, each holding its
// controller's variables and then the maps by name, with the hand-written
// edges.
func controller(p core.Promotion) *core.Diagram {
	vars := map[string][]string{}
	typed := maps.Clone(p.Types)
	if typed == nil {
		typed = map[string]types.Type{}
	}
	var mapNames []string
	for _, f := range p.Families {
		mapNames = append(mapNames, f.Map)
		typed[f.Map] = types.MapOf(f.Key, f.Val)
	}
	slices.Sort(mapNames)
	for _, mode := range p.Modes {
		vars[mode] = append(slices.Clone(p.Vars[mode]), mapNames...)
	}
	return &core.Diagram{
		States: p.Modes, Labels: p.Labels, Vars: vars, Types: typed,
		Init: p.Init, InitPost: p.InitPost, Edges: p.Edges,
		Assumes: p.Assumes, Channels: p.Channels,
	}
}

// syncLine is the line of the sync directive s, a target taking the event
// named like the sync written bare.
func syncLine(s core.Sync) (string, error) {
	targets := make([]string, len(s.Targets))
	for i, t := range s.Targets {
		targets[i] = t.Map + "(" + t.ID + ")"
		if t.Event != s.Event {
			targets[i] += "." + t.Event
		}
	}
	guard, err := clause.Print(s.Guard)
	if err != nil {
		return "", fmt.Errorf("sync %s: %w", s.Event, err)
	}
	return "sync " + s.Event + " : " + strings.Join(targets, ", ") + guarded(guard), nil
}

// guarded is what follows the targets of a sync with the guard text.
func guarded(text string) string {
	if text == "" {
		return ""
	}
	return " ; " + text
}

// includePath is p as an !include writes it, quoted if it holds a space.
func includePath(p string) string {
	if strings.ContainsAny(p, " \t") {
		return fmt.Sprintf("%q", p)
	}
	return p
}
