package coreio

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/ident"
	"github.com/Kuniwak/puml-parallel/csdf/pumlline"
	"github.com/Kuniwak/puml-parallel/csdf/types"
)

const id = ident.Pattern

var (
	stateDeclRe = pumlline.StateDeclLabelled
	edgeRe      = pumlline.Edge
	mapDeclRe   = regexp.MustCompile(`^(` + id + `)\s*:\s*(` + id + `)\s*(?:;\s*(.*?))?\s*$`)
	startRe     = pumlline.Start
	syncRe      = pumlline.Sync
	targetRe    = regexp.MustCompile(`^(` + id + `)\((` + id + `)\)(?:\.(` + id + `))?$`)
)

// ReadPromotion reads a global diagram (docs/PROMOTION.md): its states, the
// modes, each holding the maps it declares or has a <<promote>> block for;
// a block titled "<map> : (<ID> : <Key>) ⇸ <Type>" says that the family moves in that
// mode, and exactly one block of a map includes its local diagram, which
// include reads by the path written, the others being bodyless; a mode
// holding a map without a block for it has the family frozen there. Then the
// start edge into a mode, hand-written edges between modes, floating assume
// notes, and notes, floating or anchored, whose first line is a sync or
// constrain directive, or which are for the picture and are skipped, and
// floating notes of channel declarations, of the hand-written edges. Every
// variable a mode declares has a type, a map the type its block gives it.
// Clauses are read as authored.
func ReadPromotion(source string, include Include) (core.Promotion, error) {
	if err := csdf.CheckIgnoreRegions(source); err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotion: %w", err)
	}
	r := &promotionReader{include: include, holds: map[string]map[string]bool{}, moves: map[string]map[string]bool{}, declared: map[string][]string{}, types: map[string]types.Type{}}
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
	include  Include
	p        core.Promotion
	start    string
	family   *core.Family // the <<promote>> block being read
	depth    int          // how many blocks are open
	note     noteKind
	noteID   string                     // the ID of the floating note being read, "" for an anchored one
	holds    map[string]map[string]bool // the maps each mode declares or has a block for
	moves    map[string]map[string]bool // the maps each mode has a block for
	declared map[string][]string        // the variables each mode declares, in order
	types    map[string]types.Type      // the type of each variable the modes declare
	lineNo   int                        // the line being read
	edgeAt   []int                      // the line of each hand-written edge
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
	// channels is a note of channel declarations, every line of it one.
	channels
)

func (r *promotionReader) run(lines []string) error {
	for i, raw := range lines {
		r.lineNo = i + 1
		if err := r.line(strings.TrimSpace(raw)); err != nil {
			var at located
			if errors.As(err, &at) {
				return err // it names its line already
			}
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
		return r.block(m[1], m[2], m[3] != "")
	}
	if m := mapDeclRe.FindStringSubmatch(l); m != nil && (r.depth == 1 && m[1] == r.mode() || r.depth == 0 && r.holds[m[1]] != nil) {
		mode := m[1]
		if err := ident.CheckAll(m[2]); err != nil {
			return err
		}
		if err := r.declare(mode, m[2], m[3]); err != nil {
			return err
		}
		r.holds[mode][m[2]] = true
		if !slices.Contains(r.declared[mode], m[2]) {
			r.declared[mode] = append(r.declared[mode], m[2])
		}
		return nil
	}
	if m := pumlline.CompositeOpen.FindStringSubmatch(l); m != nil && r.depth == 0 {
		if err := r.newMode(m[2], pumlline.Unquote(m[1])); err != nil {
			return err
		}
		r.depth++
		return nil
	}
	if pumlline.Close.MatchString(l) && r.depth > 0 {
		if r.depth == 2 && r.family.Local == nil {
			return fmt.Errorf("the block of %s includes no local diagram", r.family.Map)
		}
		r.depth--
		return nil
	}
	if m := startRe.FindStringSubmatch(l); m != nil && r.depth == 0 {
		if post := strings.TrimSpace(m[2]); post != "" {
			// Every map starts empty; Promote drops it with a warning.
			g, err := clause.ParseAuthored(post)
			if err != nil {
				return fmt.Errorf("the start edge: %w", err)
			}
			r.p.InitPost = g
		}
		r.start = m[1]
		return nil
	}
	if m := stateDeclRe.FindStringSubmatch(l); m != nil && r.depth == 0 {
		return r.newMode(m[2], pumlline.Unquote(m[1]))
	}
	if m := edgeRe.FindStringSubmatch(l); m != nil && r.depth == 0 {
		return r.edge(l, m[1], m[2])
	}
	if m := pumlline.NoteFloating.FindStringSubmatch(l); m != nil && r.depth < 2 {
		if err := ident.CheckAll(m[1]); err != nil {
			return err
		}
		r.note, r.noteID = unread, m[1]
		return nil
	}
	if pumlline.NoteAnchored.MatchString(l) && r.depth < 2 {
		r.note, r.noteID = unread, ""
		return nil
	}
	return fmt.Errorf("unexpected %q", l)
}

// declare records that mode declares the variable v of the type written
// typ, which is required, and which is one type for every mode.
func (r *promotionReader) declare(mode, v, typ string) error {
	if typ == "" {
		return fmt.Errorf("the variable %s of %s has no type; declare it as %s : %s ; <type>, a map of a family as %s : %s ; <Key> ⇸ <Type>", v, mode, mode, v, mode, v)
	}
	t, err := types.Parse(typ)
	if err != nil {
		return fmt.Errorf("the variable %s of %s: %w", v, mode, err)
	}
	return core.DeclareType(r.types, v, t)
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
	case r.note == channels:
		return r.channelLine(l)
	case r.note == unread && (strings.HasPrefix(l, "channel ") || l == "channel"):
		if r.noteID == "" {
			return fmt.Errorf("a note of channel declarations must float: the channels are of the whole system")
		}
		r.note = channels
		return r.channelLine(l)
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
		return r.constrain(l)
	case strings.HasPrefix(l, "assume "):
		if r.noteID == "" {
			return fmt.Errorf("an assume note must float: an assumption is about the whole system")
		}
		r.p.Assumes = append(r.p.Assumes, core.Assumption{ID: r.noteID, Text: strings.TrimSpace(strings.TrimPrefix(l, "assume "))})
		return nil
	}
	m := syncRe.FindStringSubmatch(l)
	if m == nil {
		return fmt.Errorf("%q is no directive; a note holds a sync directive", l)
	}
	return r.sync(m[1], m[2], m[3])
}

// channelLine reads a line of a note of channel declarations: the
// channels of the hand-written edges.
func (r *promotionReader) channelLine(l string) error {
	if !strings.HasPrefix(l, "channel ") && l != "channel" {
		return fmt.Errorf("%q in a note of channel declarations, which holds only channel <name> : <type>.<type>", l)
	}
	decl, err := csdf.ParseChannelDecl(strings.TrimSpace(strings.TrimPrefix(l, "channel")), r.lineNo)
	if err != nil {
		return err
	}
	if _, ok := r.p.Channels[decl.Name]; ok {
		return fmt.Errorf("the channel %s is declared twice", decl.Name)
	}
	if r.p.Channels == nil {
		r.p.Channels = map[string][]types.Type{}
	}
	r.p.Channels[decl.Name] = decl.Fields
	return nil
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
	if err := ident.CheckAll(event); err != nil {
		return fmt.Errorf("sync %s: %w", event, err)
	}
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
		if err := ident.CheckAll(slices.DeleteFunc([]string{m[1], m[2], m[3]}, isLeftOut)...); err != nil {
			return fmt.Errorf("sync %s: %w", event, err)
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

// constrain reads `constrain EVENT ; guard` (docs/PROMOTION.md).
func (r *promotionReader) constrain(l string) error {
	head, guard, ok := strings.Cut(strings.TrimPrefix(l, "constrain "), ";")
	head = strings.TrimSpace(head)
	if !ok {
		return fmt.Errorf("constrain %s: expected `constrain EVENT ; guard`", head)
	}
	if strings.TrimSpace(guard) == "" {
		return fmt.Errorf("constrain %s has an empty guard, which constrains nothing", head)
	}
	e, err := csdf.ParseEvent(head)
	if err != nil {
		return fmt.Errorf("constrain %s: %w", head, err)
	}
	g, err := clause.ParseAuthored(strings.TrimSpace(guard))
	if err != nil {
		return fmt.Errorf("constrain %s: %w", head, err)
	}
	r.p.Constrains = append(r.p.Constrains, core.Constrain{Event: eventOf(e), Guard: g})
	return nil
}

// mode is the global state being read, or "" before it.
func (r *promotionReader) mode() string {
	if len(r.p.Modes) == 0 {
		return ""
	}
	return r.p.Modes[len(r.p.Modes)-1]
}

// newMode starts the mode named alias, labelled label.
func (r *promotionReader) newMode(alias, label string) error {
	if err := ident.CheckAll(alias); err != nil {
		return err
	}
	if slices.Contains(r.p.Modes, alias) {
		return fmt.Errorf("a second state %s", alias)
	}
	r.p.Modes = append(r.p.Modes, alias)
	if label != alias {
		if r.p.Labels == nil {
			r.p.Labels = map[string]string{}
		}
		r.p.Labels[alias] = label
	}
	r.holds[alias], r.moves[alias] = map[string]bool{}, map[string]bool{}
	return nil
}

// block reads a <<promote>> block of the mode being read, titled title:
// the family of its map moves there, and the block with a body includes the
// local diagram.
func (r *promotionReader) block(title, alias string, body bool) error {
	t := pumlline.PromoteTitle.FindStringSubmatch(title)
	if u := pumlline.PromoteTitleUntyped.FindStringSubmatch(title); t == nil && u != nil {
		return fmt.Errorf("the <<promote>> block %s: its title %q gives the IDs no type; write it as \"%s : (%s : <type of the IDs>) ⇸ %s\"", alias, title, u[1], u[2], u[3])
	}
	if t == nil {
		return fmt.Errorf("expected a <<promote>> title of the form \"<map> : (<ID> : <Key>) ⇸ <Type>\", got %q", title)
	}
	if err := ident.CheckAll(alias, t[1], t[2]); err != nil {
		return fmt.Errorf("the <<promote>> block %s: %w", alias, err)
	}
	key, err := types.Parse(t[3])
	if err != nil {
		return fmt.Errorf("the <<promote>> block %s: the type of the IDs: %w", alias, err)
	}
	val, err := types.Parse(t[4])
	if err != nil {
		return fmt.Errorf("the <<promote>> block %s: the type of an instance: %w", alias, err)
	}
	mode := r.mode()
	if r.moves[mode][t[1]] {
		return fmt.Errorf("a second <<promote>> block of %s in %s; a map has one block per mode", t[1], mode)
	}
	f, err := r.familyOf(t[1])
	if err != nil {
		r.p.Families = append(r.p.Families, core.Family{Map: t[1], ID: t[2], Key: key, Val: val})
		f = &r.p.Families[len(r.p.Families)-1]
	} else if f.ID != t[2] {
		return fmt.Errorf("%s: the blocks of %s write its ID as %s and as %s", alias, t[1], f.ID, t[2])
	} else if m, n := types.MapOf(f.Key, f.Val), types.MapOf(key, val); !m.Equal(n) {
		return fmt.Errorf("%s: the blocks of %s give it the type %s and the type %s", alias, t[1], m, n)
	}
	r.holds[mode][t[1]], r.moves[mode][t[1]] = true, true
	if body {
		if f.Local != nil {
			return fmt.Errorf("a second <<promote>> block of %s with a body; one block includes its local diagram", t[1])
		}
		r.family = f
		r.depth++
	}
	return nil
}

// notYetEnd says why a global end edge is refused: in CSP the promoted
// term terminates only when every process of it does, the instances among
// them, and they do not, so what the controller's termination means is
// not defined yet (docs/CLAUSE_DESIGN.md).
const notYetEnd = "is not supported yet by the formula promotion: what the controller's termination means beside instances that never terminate is not defined yet (#76)"

// edge reads a hand-written edge between modes, l, from src to dst.
func (r *promotionReader) edge(l, src, dst string) error {
	name := src + " --> " + dst
	if dst == "[*]" {
		return fmt.Errorf("%s: a global end edge %s", name, notYetEnd)
	}
	e, err := readEdge(l, r.lineNo, clause.ParseAuthored)
	if err != nil {
		return located{fmt.Errorf("%s: %w", name, err)}
	}
	r.p.Edges = append(r.p.Edges, e)
	r.edgeAt = append(r.edgeAt, r.lineNo)
	return nil
}

// isLeftOut reports whether s is an optional part a line leaves out.
func isLeftOut(s string) bool { return s == "" }

// located is an error that names its source line itself.
type located struct{ error }

func (l located) Unwrap() error { return l.error }

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
	case len(r.p.Modes) == 0:
		return fmt.Errorf("no composite state holding the families")
	case r.depth != 0 || r.note != noNote:
		return fmt.Errorf("a block or a note is not closed")
	case !slices.Contains(r.p.Modes, r.start):
		return fmt.Errorf("no start edge into a mode")
	}
	r.p.Init = r.start
	for i, e := range r.p.Edges {
		for _, s := range []string{e.Src, e.Dst} {
			if !slices.Contains(r.p.Modes, s) {
				return fmt.Errorf("line %d: %s --> %s names %s, which is no state of the global diagram", r.edgeAt[i], e.Src, e.Dst, s)
			}
		}
		decl, ok := r.p.Channels[e.Event.Channel]
		switch {
		case e.Event.Channel == core.Tau:
		case !ok && len(e.Event.Fields) > 0:
			return fmt.Errorf("line %d: the channel %s carries fields, and no channel note declares their types; declare it as channel %s : <type>", r.edgeAt[i], e.Event.Channel, e.Event.Channel)
		case ok && len(decl) != len(e.Event.Fields):
			return fmt.Errorf("line %d: %s has %d fields, but the channel %s is declared with %d", r.edgeAt[i], e.Event.Label(), len(e.Event.Fields), e.Event.Channel, len(decl))
		}
	}
	for _, mode := range r.p.Modes {
		for _, v := range r.declared[mode] {
			if f, err := r.familyOf(v); err == nil {
				if want := types.MapOf(f.Key, f.Val); !r.types[v].Equal(want) {
					return fmt.Errorf("%s declares the map %s of the type %s, but its <<promote>> block gives it the type %s", mode, v, r.types[v], want)
				}
				continue
			}
			// A variable that is no family's map is the controller's own
			// (docs/CLAUSE_DESIGN.md).
			if r.p.Vars == nil {
				r.p.Vars = map[string][]string{}
			}
			r.p.Vars[mode] = append(r.p.Vars[mode], v)
			if r.p.Types == nil {
				r.p.Types = map[string]types.Type{}
			}
			r.p.Types[v] = r.types[v]
		}
	}
	for i := range r.p.Families {
		f := &r.p.Families[i]
		if f.Local == nil {
			return fmt.Errorf("no block of %s includes its local diagram", f.Map)
		}
		for _, mode := range r.p.Modes {
			switch {
			case !r.holds[mode][f.Map]:
				return fmt.Errorf("%s holds no map %s: every mode holds every map, the instances existing whatever the mode", mode, f.Map)
			case !r.moves[mode][f.Map]:
				f.Frozen = append(f.Frozen, mode)
			}
		}
	}
	return nil
}
