package coreio

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf"
	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// WrittenPromotion is a global diagram as written: what ReadPromotion reads,
// but for the local diagrams, which it does not load, with the names that
// are for the reader alone, the aliases of the blocks and the IDs of the
// notes, and the order the modes declare their variables in. It is for
// printing (WritePromotion) and cannot be expanded.
type WrittenPromotion struct {
	p            core.Promotion // its families have no local diagram
	declared     map[string][]string
	types        map[string]types.Type
	aliases      map[string]map[string]string
	includes     map[string]string
	includeIn    map[string]string
	syncIDs      []string
	constrainIDs []string
}

// ReadPromotionAsWritten reads the global diagram source as ReadPromotion
// does, but reads none of the files it includes, keeping the paths instead.
func ReadPromotionAsWritten(source string) (WrittenPromotion, error) {
	if err := csdf.CheckIgnoreRegions(source); err != nil {
		return WrittenPromotion{}, fmt.Errorf("coreio.ReadPromotionAsWritten: %w", err)
	}
	r, err := readPromotion(source, nil)
	if err != nil {
		return WrittenPromotion{}, fmt.Errorf("coreio.ReadPromotionAsWritten: %w", err)
	}
	return WrittenPromotion{
		p: r.p, declared: r.declared, types: r.types, aliases: r.aliases,
		includes: r.includes, includeIn: r.includeIn, syncIDs: r.syncIDs, constrainIDs: r.constrainIDs,
	}, nil
}

// WritePromotion writes the global diagram w as written, without expanding
// it, in a canonical order, so that what it prints depends on the diagram
// and not on how it was authored: the modes by ID, each with the variables
// it declares in their order and its <<promote>> blocks by map, the
// !include in the block that has it; the start edge, the hand-written
// edges as Canonical orders them, the sync notes by event, the constrain
// notes by their line, the assume notes and the channels. Aliases and note
// IDs are kept; a note written anchored floats, under the ID sync_<event>
// or constrain<n>, n its place among the constrain notes, and an ID so made
// that is taken already is an error. Comments, notes for the picture, prose
// under a directive and the preprocessor's lines are left out.
func WritePromotion(w WrittenPromotion) (string, error) {
	d, err := Canonical(w.controller())
	if err != nil {
		return "", fmt.Errorf("coreio.WritePromotion: %w", err)
	}
	syncs, constrains, err := w.directives()
	if err != nil {
		return "", fmt.Errorf("coreio.WritePromotion: %w", err)
	}
	mapNames := slices.Sorted(maps.Keys(w.includes))
	var b strings.Builder
	b.WriteString("@startuml" + titled(w.p.Name) + "\n")
	for _, mode := range d.States {
		label, ok := d.Labels[mode]
		if !ok {
			label = mode
		}
		fmt.Fprintf(&b, "state %q as %s {\n", label, mode)
		writeVars(&b, "  ", d, mode)
		for _, m := range mapNames {
			alias, ok := w.aliases[mode][m]
			if !ok {
				continue
			}
			f := w.family(m)
			fmt.Fprintf(&b, "  state %q as %s <<promote>>", fmt.Sprintf("%s : (%s : %s) ⇸ %s", f.Map, f.ID, f.Key, f.Val), alias)
			if w.includeIn[m] != mode {
				b.WriteString("\n")
				continue
			}
			fmt.Fprintf(&b, " {\n    !include %s\n  }\n", includePath(w.includes[m]))
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
	for _, n := range append(syncs, constrains...) {
		fmt.Fprintf(&b, "note as %s\n  %s\nend note\n", n.id, n.line)
	}
	for _, a := range d.Assumes {
		fmt.Fprintf(&b, "note as %s\n  assume %s\nend note\n", a.ID, a.Text)
	}
	writeChannels(&b, d.Channels)
	b.WriteString("@enduml\n")
	return b.String(), nil
}

// controller is the diagram of w's modes as states, each holding the
// variables it declares, with the hand-written edges.
func (w WrittenPromotion) controller() *core.Diagram {
	p := w.p
	return &core.Diagram{
		States: p.Modes, Labels: p.Labels, Vars: w.declared, Types: w.types,
		Init: p.Init, InitPost: p.InitPost, Edges: p.Edges,
		Assumes: p.Assumes, Channels: p.Channels,
	}
}

// family is the family of the map m.
func (w WrittenPromotion) family(m string) core.Family {
	i := slices.IndexFunc(w.p.Families, func(f core.Family) bool { return f.Map == m })
	return w.p.Families[i]
}

// note is a directive note: its ID and its line.
type note struct{ id, line string }

// directives are the sync notes of w by event and its constrain notes by
// line, each under the ID written or, for an anchored note, one made.
func (w WrittenPromotion) directives() ([]note, []note, error) {
	taken := map[string]bool{"channels": len(w.p.Channels) > 0}
	for _, id := range slices.Concat(w.p.Modes, w.syncIDs, w.constrainIDs) {
		taken[id] = true
	}
	for _, byMap := range w.aliases {
		for _, a := range byMap {
			taken[a] = true
		}
	}
	for _, a := range w.p.Assumes {
		taken[a.ID] = true
	}
	made := func(id string) (string, error) {
		if taken[id] {
			return "", fmt.Errorf("an anchored note would float as %s, which names something else already; give the note an ID of its own", id)
		}
		taken[id] = true
		return id, nil
	}
	syncs := make([]note, len(w.p.Syncs))
	for i, s := range w.p.Syncs {
		line, err := syncLine(s)
		if err != nil {
			return nil, nil, err
		}
		syncs[i] = note{w.syncIDs[i], line}
	}
	slices.SortFunc(syncs, func(a, b note) int { return cmp.Compare(a.line, b.line) })
	constrains := make([]note, len(w.p.Constrains))
	for i, c := range w.p.Constrains {
		guard, err := clause.Print(c.Guard)
		if err != nil {
			return nil, nil, fmt.Errorf("constrain %s: %w", event(c.Event), err)
		}
		constrains[i] = note{w.constrainIDs[i], "constrain " + event(c.Event) + " ; " + guard}
	}
	slices.SortFunc(constrains, func(a, b note) int { return cmp.Compare(a.line, b.line) })
	for i := range syncs {
		if syncs[i].id != "" {
			continue
		}
		event, _, _ := strings.Cut(strings.TrimPrefix(syncs[i].line, "sync "), " ")
		id, err := made("sync_" + event)
		if err != nil {
			return nil, nil, err
		}
		syncs[i].id = id
	}
	for i := range constrains {
		if constrains[i].id != "" {
			continue
		}
		id, err := made(fmt.Sprintf("constrain%d", i+1))
		if err != nil {
			return nil, nil, err
		}
		constrains[i].id = id
	}
	return syncs, constrains, nil
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
