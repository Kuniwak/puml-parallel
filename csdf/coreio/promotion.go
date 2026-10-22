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
	stateDeclRe = regexp.MustCompile(`^state\s+"(?:[^"\\]|\\.)*"\s+as\s+(` + id + `)$`)
	edgeRe      = regexp.MustCompile(`^(` + id + `)\s*-->\s*(` + id + `|\[\*\])`)
	mapDeclRe   = regexp.MustCompile(`^(` + id + `)\s*:\s*(` + id + `)\s*(?:;.*)?$`)
	startRe     = regexp.MustCompile(`^\[\*\]\s*-->\s*(` + id + `)\s*(?::\s*(.*))?$`)
	syncRe      = regexp.MustCompile(`^sync\s+(` + id + `)\s*:\s*([^;]*?)\s*(?:;\s*(.*))?$`)
	targetRe    = regexp.MustCompile(`^(` + id + `)\((` + id + `)\)(?:\.(` + id + `))?$`)
)

// ReadPromotion reads a global diagram (docs/PROMOTION.md): one composite
// state holding a <<promote>> block per family, titled
// "<map> : <ID> ⇸ <Type>" and including its local diagram, which include
// reads by the path written, and the declarations of the maps it holds; a
// start edge into that state; and notes, floating or anchored, whose first
// line is a sync directive, or which are for the picture and are skipped.
// The type in a title is not read yet. The clauses of the local diagrams and
// of the syncs are read as authored.
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

// notYet refuses what a formula promotion does not read yet, by name: the
// parts of docs/PROMOTION.md that step 5½b of #76 brings over.
func notYet(what string) error {
	return fmt.Errorf("%s is not supported yet by the formula promotion (step 5½b of #76)", what)
}

// Include is the source of the file an !include names, by the path written.
type Include func(path string) (string, error)

type promotionReader struct {
	include Include
	p       core.Promotion
	start   string
	family  *core.Family // the <<promote>> block being read
	depth   int          // how many blocks are open
	note    noteKind
	maps    []string // the maps the global state declares
}

// noteKind is what the note being read is, once its first line says.
type noteKind int

const (
	noNote noteKind = iota
	// unread is a note whose first line is yet to come.
	unread
	// directed is a note whose first line was a directive; its other lines
	// are prose.
	directed
	// picture is a note for the picture, skipped.
	picture
)

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
	case r.note != noNote:
		return r.noteLine(l)
	case strings.HasPrefix(l, "!include"):
		return r.includeLine(l)
	case strings.HasPrefix(l, "!"):
		return nil // PlantUML's preprocessor, for the picture
	}
	if m := pumlline.Promote.FindStringSubmatch(l); m != nil && r.depth == 1 {
		if m[3] == "" {
			// It says that a family moves in a second global state.
			return fmt.Errorf("%s: %w", m[2], notYet("a bodyless <<promote>> block"))
		}
		title := pumlline.PromoteTitle.FindStringSubmatch(m[1])
		if title == nil {
			return fmt.Errorf("expected a <<promote>> title of the form \"<map> : <ID> ⇸ <Type>\", got %q", m[1])
		}
		if _, err := r.familyOf(title[1]); err == nil {
			return fmt.Errorf("a second <<promote>> block of %s; a map has one block", title[1])
		}
		r.p.Families = append(r.p.Families, core.Family{Map: title[1], ID: title[2]})
		r.family = &r.p.Families[len(r.p.Families)-1]
		r.depth++
		return nil
	}
	if m := mapDeclRe.FindStringSubmatch(l); m != nil && r.depth == 1 && m[1] == r.p.State {
		r.maps = append(r.maps, m[2])
		return nil
	}
	if m := pumlline.CompositeOpen.FindStringSubmatch(l); m != nil && r.depth == 0 {
		if r.p.State != "" {
			return fmt.Errorf("%s: %w", m[2], notYet("a second global state"))
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
			// Every map starts empty, which is all such a post can say.
			return notYet("a post on the start edge")
		}
		r.start = m[1]
		return nil
	}
	if m := stateDeclRe.FindStringSubmatch(l); m != nil && r.depth == 0 {
		return fmt.Errorf("%s: %w", m[1], notYet("a second global state"))
	}
	if edgeRe.MatchString(l) && r.depth == 0 {
		return notYet("a hand-written edge of the global diagram")
	}
	if (pumlline.NoteFloating.MatchString(l) || pumlline.NoteAnchored.MatchString(l)) && r.depth < 2 {
		r.note = unread
		return nil
	}
	return fmt.Errorf("unexpected %q", l)
}

func (r *promotionReader) includeLine(l string) error {
	if word := strings.Fields(l)[0]; word != "!include" {
		return fmt.Errorf("%s is not read: a <<promote>> block includes its local diagram with !include", word)
	}
	m := pumlline.Include.FindStringSubmatch(l)
	if m == nil || r.depth != 2 || r.family.Local != nil {
		return fmt.Errorf("an !include belongs in a <<promote>> block, once")
	}
	src, err := r.include(pumlline.Unquote(strings.TrimSpace(m[1])))
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

// noteLine reads a line of a note. Its first line decides what the note is
// (docs/PROMOTION.md): a directive, or a note for the picture.
func (r *promotionReader) noteLine(l string) error {
	switch {
	case pumlline.NoteEnd.MatchString(l):
		r.note = noNote
		return nil
	case r.note == picture:
		return nil
	case r.note == directed && isDirective(l):
		return fmt.Errorf("this note holds a second directive; write one note per directive")
	case r.note == directed:
		return nil // prose under the directive
	case !isDirective(l):
		r.note = picture
		return nil
	}
	r.note = directed
	switch {
	case strings.HasPrefix(l, "constrain "):
		return notYet("constrain")
	case strings.HasPrefix(l, "assume "):
		return notYet("an assume note in the global diagram")
	}
	m := syncRe.FindStringSubmatch(l)
	if m == nil {
		return fmt.Errorf("%q is no directive; a note holds a sync directive", l)
	}
	return r.sync(m[1], m[2], m[3])
}

// isDirective reports whether a note line opens a directive, the core's
// assume included (docs/PROMOTION.md).
func isDirective(l string) bool {
	for _, w := range []string{"sync ", "constrain ", "assume "} {
		if strings.HasPrefix(l, w) {
			return true
		}
	}
	return false
}

func (r *promotionReader) sync(event, targets, guard string) error {
	for _, other := range r.p.Syncs {
		if other.Event == event {
			return fmt.Errorf("a second sync of %s; one event is synchronised once", event)
		}
	}
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
	case r.depth != 0 || r.note != noNote:
		return fmt.Errorf("a block or a note is not closed")
	case r.start != r.p.State:
		return fmt.Errorf("no start edge into %s", r.p.State)
	}
	for _, m := range r.maps {
		if _, err := r.familyOf(m); err != nil {
			return fmt.Errorf("%s: %w", m, notYet("a variable of the global state that is no family's map"))
		}
	}
	return nil
}
