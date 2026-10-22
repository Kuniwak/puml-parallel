package coreio

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/ident"
	"github.com/Kuniwak/puml-parallel/csdf/pumlline"
)

const id = ident.Pattern

var (
	familyOpenRe = regexp.MustCompile(`^state\s+"\s*(` + id + `)\s*:\s*(` + id + `)\s*"\s+as\s+` + id + `\s+<<promote>>\s*\{$`)
	startRe      = regexp.MustCompile(`^\[\*\]\s*-->\s*(` + id + `)\s*(?::\s*(.*))?$`)
	syncRe       = regexp.MustCompile(`^sync\s+(` + id + `)\s*:\s*([^;]*?)\s*(?:;\s*(.*))?$`)
	targetRe     = regexp.MustCompile(`^(` + id + `)\((` + id + `)\)(?:\.(` + id + `))?$`)
)

// ReadPromotion reads a global diagram (docs/SYNTAX.md, "Promotion"): one
// composite state holding a <<promote>> block per family, each including its
// local diagram, which include reads by the path written; a start edge into
// that state; and notes holding sync directives. The
// clauses of the local diagrams and of the syncs are read as authored.
func ReadPromotion(source string, include Include) (core.Promotion, error) {
	r := &promotionReader{include: include}
	if err := r.run(strings.Split(csdf.BlankIgnoreRegions(source), "\n")); err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotion: %w", err)
	}
	if err := r.check(); err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotion: %w", err)
	}
	return r.p, nil
}

// Include is the source of the file an !include names, by the path written.
type Include func(path string) (string, error)

type promotionReader struct {
	include Include
	p       core.Promotion
	start   string
	family  *core.Family // the <<promote>> block being read
	depth   int          // how many blocks are open
	note    bool
}

func (r *promotionReader) run(lines []string) error {
	for i, raw := range lines {
		if err := r.line(strings.TrimSpace(raw)); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return nil
}

func (r *promotionReader) line(l string) error {
	switch {
	case l == "" || strings.HasPrefix(l, "'"), strings.HasPrefix(l, "@startuml"), l == "@enduml":
		return nil
	case r.note:
		return r.directive(l)
	case strings.HasPrefix(l, "!include"):
		return r.includeLine(l)
	case strings.HasPrefix(l, "!"):
		return nil // PlantUML's preprocessor, for the picture
	}
	if m := familyOpenRe.FindStringSubmatch(l); m != nil && r.depth == 1 {
		r.p.Families = append(r.p.Families, core.Family{Map: m[1], ID: m[2]})
		r.family = &r.p.Families[len(r.p.Families)-1]
		r.depth++
		return nil
	}
	if m := pumlline.CompositeOpen.FindStringSubmatch(l); m != nil && r.depth == 0 {
		if r.p.State != "" {
			return fmt.Errorf("a second state %s; a promoted diagram has one state", m[2])
		}
		r.p.State = m[2]
		r.depth++
		return nil
	}
	if l == "}" && r.depth > 0 {
		if r.depth == 2 && r.family.Local == nil {
			return fmt.Errorf("the block of %s includes no local diagram", r.family.Map)
		}
		r.depth--
		return nil
	}
	if m := startRe.FindStringSubmatch(l); m != nil && r.depth == 0 {
		if strings.TrimSpace(m[2]) != "" {
			return fmt.Errorf("a post on the start edge; a promoted diagram starts with every map empty")
		}
		r.start = m[1]
		return nil
	}
	if pumlline.NoteFloating.MatchString(l) && r.depth == 0 {
		r.note = true
		return nil
	}
	return fmt.Errorf("unexpected %q", l)
}

func (r *promotionReader) includeLine(l string) error {
	m := pumlline.Include.FindStringSubmatch(l)
	if m == nil || r.depth != 2 || r.family.Local != nil {
		return fmt.Errorf("an !include belongs in a <<promote>> block, once")
	}
	src, err := r.include(strings.TrimSpace(m[1]))
	if err != nil {
		return err
	}
	local, err := ReadAuthored(src)
	if err != nil {
		return fmt.Errorf("%s: %w", m[1], err)
	}
	r.family.Local = local
	return nil
}

func (r *promotionReader) directive(l string) error {
	if l == "end note" {
		r.note = false
		return nil
	}
	if m := syncRe.FindStringSubmatch(l); m != nil {
		return r.sync(m[1], m[2], m[3])
	}
	return fmt.Errorf("%q is no directive; a note holds sync directives", l)
}

func (r *promotionReader) sync(event, targets, guard string) error {
	s := core.Sync{Event: event}
	for _, t := range strings.Split(targets, ",") {
		m := targetRe.FindStringSubmatch(strings.TrimSpace(t))
		if m == nil {
			return fmt.Errorf("sync %s: %q: expected map(id) or map(id).event", event, strings.TrimSpace(t))
		}
		if _, err := r.familyOf(m[1]); err != nil {
			return fmt.Errorf("sync %s: %w", event, err)
		}
		local := m[3]
		if local == "" {
			local = event // a bare target takes the local event named like the sync
		}
		s.Targets = append(s.Targets, core.Target{Map: m[1], ID: m[2], Event: local})
	}
	g, err := clause.ParseAuthored(guard)
	if err != nil {
		return fmt.Errorf("sync %s: %w", event, err)
	}
	s.Guard = g
	r.p.Syncs = append(r.p.Syncs, s)
	return nil
}

func (r *promotionReader) familyOf(m string) (*core.Family, error) {
	for i := range r.p.Families {
		if r.p.Families[i].Map == m {
			return &r.p.Families[i], nil
		}
	}
	return nil, fmt.Errorf("no family of the map %s", m)
}

// check reports, as an error, what the whole diagram lacks.
func (r *promotionReader) check() error {
	switch {
	case r.p.State == "":
		return fmt.Errorf("no composite state holding the families")
	case r.depth != 0 || r.note:
		return fmt.Errorf("a block or a note is not closed")
	case r.start != r.p.State:
		return fmt.Errorf("no start edge into %s", r.p.State)
	}
	return nil
}
