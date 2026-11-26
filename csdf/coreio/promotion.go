package coreio

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v6/csdf"
	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v6/csdf/core"
	"github.com/Kuniwak/puml-parallel/v6/csdf/ident"
	"github.com/Kuniwak/puml-parallel/v6/csdf/pumlline"
	"github.com/Kuniwak/puml-parallel/v6/csdf/types"
)

// ReadPromotion reads a global diagram (docs/PROMOTION.md): its syntax tree
// (csdf.Parse), each !include resolved by include (ResolveIncludes), read
// by ReadPromotionTree.
func ReadPromotion(source string, include Include) (core.Promotion, error) {
	parsed, err := csdf.Parse("", source)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotion: %w", err)
	}
	tree, err := ResolveIncludes(parsed, include)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotion: %w", err)
	}
	p, err := ReadPromotionTree(tree)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotion: %w", err)
	}
	return p, nil
}

// ReadPromotionTree reads the syntax tree of a global diagram
// (docs/PROMOTION.md): its states, the modes, each holding the maps it
// declares or has a <<promote>> block for; a block titled "<map> : (<ID> :
// <Key>) ⇸ <Type>" says that the family moves in that mode, and exactly one
// block of a map includes its local diagram, the tree its resolved
// !include holds (ResolveIncludes), the others being bodyless; an
// unresolved !include is refused (UnresolvedInclude); a mode holding a map without a
// block for it has the family frozen there. Then the start edge into a
// mode, hand-written edges between modes, floating assume notes, and notes,
// floating or anchored, whose first line is a sync or constrain directive,
// or which are for the picture and are skipped, and floating notes of
// channel declarations, of the hand-written edges. Every variable a mode
// declares has a type, a map the type its block gives it. Clauses are read
// as authored.
func ReadPromotionTree(tree *csdf.SyntaxTree) (core.Promotion, error) {
	r, err := readPromotion(tree, lowerIncluded)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotionTree: %w", err)
	}
	return r.p, nil
}

// readPromotion reads tree as ReadPromotionTree does, each local diagram
// read by local, the reader kept for what it saw as written.
func readPromotion(tree *csdf.SyntaxTree, local localDiagram) (*promotionReader, error) {
	r := &promotionReader{local: local, holds: map[string]map[string]bool{}, moves: map[string]map[string]bool{}, declared: map[string][]string{}, types: map[string]types.Type{}, aliases: map[string]map[string]string{}, includes: map[string]string{}, includeIn: map[string]string{}}
	r.p.Name = tree.Name
	if err := r.body(tree.Body); err != nil {
		return nil, err
	}
	if err := r.check(); err != nil {
		return nil, err
	}
	return r, nil
}

// localDiagram reads the local diagram the !include p of a <<promote>>
// block names; the Diagram it returns is nil when it reads none.
type localDiagram func(p *csdf.PragmaSyntax) (*core.Diagram, error)

// lowerIncluded is the local diagram of the tree p's resolved !include
// holds, read as authored; an unresolved !include is refused.
func lowerIncluded(p *csdf.PragmaSyntax) (*core.Diagram, error) {
	if p.Tree == nil {
		return nil, UnresolvedInclude(pumlline.Unquote(p.Arg))
	}
	d, err := LowerAuthored(p.Tree)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.Arg, err)
	}
	return d, nil
}

// leaveUnread reads no local diagram, for a promotion read as written.
func leaveUnread(*csdf.PragmaSyntax) (*core.Diagram, error) { return nil, nil }

// Include reads the file an !include names, by the path written: its
// source, and the path of the file read.
type Include func(path string) (source, read string, err error)

type promotionReader struct {
	local    localDiagram
	p        core.Promotion
	start    string
	family   *core.Family               // the <<promote>> block being read
	depth    int                        // how many blocks are open
	noteID   string                     // the ID of the floating note being read, "" for an anchored one
	holds    map[string]map[string]bool // the maps each mode declares or has a block for
	moves    map[string]map[string]bool // the maps each mode has a block for
	declared map[string][]string        // the variables each mode declares, in order
	types    map[string]types.Type      // the type of each variable the modes declare
	lineNo   int                        // the line being read
	edgeAt   []int                      // the line of each hand-written edge
	// What is for the reader alone, kept for printing the diagram as
	// written: the alias of each block, by mode and map, the path each map
	// includes and the mode of the block that includes it, and the ID of
	// the note of each sync and constrain, "" for an anchored one.
	aliases      map[string]map[string]string
	includes     map[string]string
	includeIn    map[string]string
	syncIDs      []string
	constrainIDs []string
}

// body reads the elements of a body, each refusal naming the line of its
// element.
func (r *promotionReader) body(es []csdf.Element) error {
	for _, e := range es {
		r.lineNo = e.Position().Line
		if err := r.element(e); err != nil {
			var at located
			if errors.As(err, &at) {
				return err // it names its line already
			}
			return located{fmt.Errorf("line %d: %w", e.Position().Line, err)}
		}
	}
	return nil
}

func (r *promotionReader) element(e csdf.Element) error {
	switch e := e.(type) {
	case *csdf.CommentSyntax, *csdf.IgnoreSyntax, *csdf.ObserveSyntax:
		return nil
	case *csdf.PragmaSyntax:
		return r.pragma(e)
	case *csdf.StateSyntax:
		return r.state(e)
	case *csdf.StateVarSyntax:
		if r.depth == 1 && e.State == r.mode() || r.depth == 0 && r.holds[e.State] != nil {
			return r.variable(e)
		}
		return fmt.Errorf("unexpected %s : %s", e.State, e.Name)
	case *csdf.StartEdgeSyntax:
		if r.depth == 0 {
			return r.startEdge(e)
		}
	case *csdf.EdgeSyntax:
		if r.depth == 0 {
			return r.edge(e)
		}
	case *csdf.EndEdgeSyntax:
		if r.depth == 0 {
			return fmt.Errorf("%s --> [*]: a global end edge %s", e.Src, notYetEnd)
		}
	case *csdf.NoteSyntax:
		if r.depth < 2 {
			return r.note(e)
		}
	}
	return fmt.Errorf("unexpected %s", elementName(e))
}

// elementName names e in a refusal.
func elementName(e csdf.Element) string {
	switch e := e.(type) {
	case *csdf.StartEdgeSyntax:
		return "[*] --> " + e.Dst
	case *csdf.EdgeSyntax:
		return e.Src + " --> " + e.Dst
	case *csdf.EndEdgeSyntax:
		return e.Src + " --> [*]"
	case *csdf.NoteSyntax:
		return "note"
	}
	return "element"
}

// pragma reads a line of PlantUML's preprocessor: the !include of a
// <<promote>> block, or another for the picture.
func (r *promotionReader) pragma(p *csdf.PragmaSyntax) error {
	switch {
	case p.Name == "include":
		return r.includeLine(p)
	case strings.HasPrefix(p.Name, "include"):
		return fmt.Errorf("!%s is not read: a <<promote>> block includes its local diagram with !include", p.Name)
	}
	return nil // for the picture
}

// state reads a state: a mode, plain or composite, or a <<promote>> block
// of the mode being read.
func (r *promotionReader) state(s *csdf.StateSyntax) error {
	switch {
	case s.Stereotype == "promote" && r.depth == 1:
		if err := r.block(s.Label, s.ID, s.Composite); err != nil {
			return err
		}
		if !s.Composite {
			return nil
		}
		if err := r.body(s.Body); err != nil {
			return err
		}
		if r.includes[r.family.Map] == "" {
			return fmt.Errorf("the block of %s includes no local diagram", r.family.Map)
		}
		r.depth--
		return nil
	case s.Stereotype == "" && r.depth == 0:
		if err := r.newMode(s.ID, s.Label); err != nil {
			return err
		}
		if !s.Composite {
			return nil
		}
		r.depth++
		if err := r.body(s.Body); err != nil {
			return err
		}
		r.depth--
		return nil
	}
	return fmt.Errorf("unexpected state %s", s.ID)
}

// variable reads a variable a mode declares.
func (r *promotionReader) variable(v *csdf.StateVarSyntax) error {
	mode := v.State
	if err := ident.CheckAll(v.Name); err != nil {
		return err
	}
	if err := r.declare(mode, v.Name, v.Type); err != nil {
		return err
	}
	r.holds[mode][v.Name] = true
	if !slices.Contains(r.declared[mode], v.Name) {
		r.declared[mode] = append(r.declared[mode], v.Name)
	}
	return nil
}

// startEdge reads the start edge into a mode.
func (r *promotionReader) startEdge(e *csdf.StartEdgeSyntax) error {
	if e.Post != nil {
		// Every map starts empty; Promote drops it with a warning.
		g, err := clause.AuthoredFormulaOf(e.Post.Syntax)
		if err != nil {
			return fmt.Errorf("the start edge: %w", err)
		}
		r.p.InitPost = g
	}
	r.start = e.Dst
	return nil
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

func (r *promotionReader) includeLine(p *csdf.PragmaSyntax) error {
	if r.depth != 2 || r.includes[r.family.Map] != "" {
		return fmt.Errorf("an !include belongs in a <<promote>> block, once")
	}
	path := pumlline.Unquote(p.Arg)
	r.includes[r.family.Map], r.includeIn[r.family.Map] = path, r.mode()
	local, err := r.local(p)
	if err != nil {
		return err
	}
	r.family.Local = local
	return nil
}

// note reads a note. Its first line decides what the note is
// (docs/PROMOTION.md): a directive, a note of channel declarations, or a
// note for the picture.
func (r *promotionReader) note(n *csdf.NoteSyntax) error {
	r.noteID = n.ID
	if len(n.Lines) == 0 {
		return nil
	}
	switch n.Lines[0].(type) {
	case *csdf.ProseSyntax:
		return nil // for the picture
	case *csdf.ChannelSyntax:
		if r.noteID == "" {
			return fmt.Errorf("a note of channel declarations must float: the channels are of the whole system")
		}
		for _, l := range n.Lines {
			if err := r.noteLine(l, r.channelLine); err != nil {
				return err
			}
		}
		return nil
	}
	if err := r.noteLine(n.Lines[0], r.directive); err != nil {
		return err
	}
	for _, l := range n.Lines[1:] {
		if _, ok := l.(*csdf.ProseSyntax); !ok {
			return r.noteLine(l, func(csdf.Element) error {
				return fmt.Errorf("this note holds a second directive; write one note per directive")
			})
		}
	}
	return nil
}

// noteLine reads l, a line of a note, by read, a refusal naming its line.
func (r *promotionReader) noteLine(l csdf.Element, read func(csdf.Element) error) error {
	r.lineNo = l.Position().Line
	if err := read(l); err != nil {
		return located{fmt.Errorf("line %d: %w", r.lineNo, err)}
	}
	return nil
}

// directive reads the directive a note opens with.
func (r *promotionReader) directive(l csdf.Element) error {
	switch l := l.(type) {
	case *csdf.ConstrainSyntax:
		return r.constrain(l)
	case *csdf.AssumeSyntax:
		if r.noteID == "" {
			return fmt.Errorf("an assume note must float: an assumption is about the whole system")
		}
		r.p.Assumes = append(r.p.Assumes, core.Assumption{ID: r.noteID, Text: l.Text})
		return nil
	case *csdf.SyncSyntax:
		return r.sync(l)
	}
	return fmt.Errorf("a note holds one directive: sync, constrain or assume")
}

// channelLine reads a line of a note of channel declarations: the
// channels of the hand-written edges.
func (r *promotionReader) channelLine(l csdf.Element) error {
	c, ok := l.(*csdf.ChannelSyntax)
	if !ok {
		return fmt.Errorf("%s in a note of channel declarations, which holds only channel <name> : <type>.<type>", lineText(l))
	}
	decl, err := csdf.ParseChannelDecl(c.Text, r.lineNo)
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

// lineText names a line of a note in a refusal.
func lineText(l csdf.Element) string {
	if p, ok := l.(*csdf.ProseSyntax); ok {
		return fmt.Sprintf("%q", p.Text)
	}
	return "a directive"
}

// guardOf is the formula of a guard written as authored, true when it is
// not written.
func guardOf(g *csdf.ClauseSyntax) (clause.And, error) {
	if g == nil {
		return clause.And{}, nil
	}
	return clause.AuthoredFormulaOf(g.Syntax)
}

func (r *promotionReader) sync(d *csdf.SyncSyntax) error {
	event := d.Event
	if err := ident.CheckAll(event); err != nil {
		return fmt.Errorf("sync %s: %w", event, err)
	}
	for _, other := range r.p.Syncs {
		if other.Event == event {
			return fmt.Errorf("a second sync of %s; one event is synchronised once", event)
		}
	}
	s := core.Sync{Event: event}
	for _, t := range d.Targets {
		if err := ident.CheckAll(slices.DeleteFunc([]string{t.Map, t.ID, t.Event}, isLeftOut)...); err != nil {
			return fmt.Errorf("sync %s: %w", event, err)
		}
		if _, err := r.familyOf(t.Map); err != nil {
			return fmt.Errorf("sync %s: %w", event, err)
		}
		local := t.Event
		if local == "" {
			local = event // a bare target takes the local event named like the sync
		}
		s.Targets = append(s.Targets, core.Target{Map: t.Map, ID: t.ID, Event: local})
	}
	g, err := guardOf(d.Guard)
	if err != nil {
		return fmt.Errorf("sync %s: %w", event, err)
	}
	s.Guard = g
	r.p.Syncs = append(r.p.Syncs, s)
	r.syncIDs = append(r.syncIDs, r.noteID)
	return nil
}

// constrain reads `constrain EVENT ; guard` (docs/PROMOTION.md).
func (r *promotionReader) constrain(c *csdf.ConstrainSyntax) error {
	head := c.Event.Text
	if c.Guard == nil {
		return fmt.Errorf("constrain %s: expected `constrain EVENT ; guard`", head)
	}
	if strings.TrimSpace(c.Guard.Text) == "" {
		return fmt.Errorf("constrain %s has an empty guard, which constrains nothing", head)
	}
	g, err := guardOf(c.Guard)
	if err != nil {
		return fmt.Errorf("constrain %s: %w", head, err)
	}
	r.p.Constrains = append(r.p.Constrains, core.Constrain{Event: eventOf(csdf.PlainEdge(&csdf.EdgeSyntax{Event: c.Event}).Event), Guard: g})
	r.constrainIDs = append(r.constrainIDs, r.noteID)
	return nil
}

// edge reads a hand-written edge between modes.
func (r *promotionReader) edge(s *csdf.EdgeSyntax) error {
	e, err := edgeOf(csdf.PlainEdge(s), clause.AuthoredFormulaOf)
	if err != nil {
		return located{fmt.Errorf("%s --> %s: %w", s.Src, s.Dst, err)}
	}
	r.p.Edges = append(r.p.Edges, e)
	r.edgeAt = append(r.edgeAt, r.lineNo)
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
	if r.aliases[mode] == nil {
		r.aliases[mode] = map[string]string{}
	}
	r.aliases[mode][t[1]] = alias
	if f.Blocks == nil {
		f.Blocks = map[string]string{}
	}
	f.Blocks[mode] = alias
	if body {
		if r.includes[f.Map] != "" {
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
	if err := r.bindersApart(); err != nil {
		return err
	}
	for i := range r.p.Families {
		f := &r.p.Families[i]
		if r.includes[f.Map] == "" {
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

// bindersApart reports, as an error, an ∃ of a clause of the global diagram
// that binds a name a clause of it could read, as core.Diagram.BindersApart
// does of a local diagram, but with every variable of the global diagram
// taken on every clause, whatever its mode: a map, which every mode holds,
// or the controller's own, a field of the event, an ID of a sync, or an
// environment function a clause applies.
func (r *promotionReader) bindersApart() error {
	clauses := []clause.And{r.p.InitPost}
	for _, e := range r.p.Edges {
		clauses = append(clauses, e.Guard, e.Post)
	}
	for _, s := range r.p.Syncs {
		clauses = append(clauses, s.Guard)
	}
	for _, c := range r.p.Constrains {
		clauses = append(clauses, c.Guard)
	}
	var fns []string
	for _, c := range clauses {
		fns = append(fns, clause.FunctionNames(c)...)
	}
	global := func(fields, ids []string) func(string) string {
		return func(h string) string {
			switch {
			case slices.ContainsFunc(r.p.Families, func(f core.Family) bool { return f.Map == h }),
				slices.ContainsFunc(r.p.Modes, func(m string) bool { return slices.Contains(r.p.Vars[m], h) }):
				return "a state variable of the global diagram"
			case slices.Contains(fields, h):
				return "a field of the event"
			case slices.Contains(ids, h):
				return "an ID of the sync"
			case slices.Contains(fns, h):
				return "an environment function"
			}
			return ""
		}
	}
	if err := core.BindersApartFrom(r.p.InitPost, global(nil, nil)); err != nil {
		return fmt.Errorf("the start edge: %w", err)
	}
	for i, e := range r.p.Edges {
		for _, c := range []clause.And{e.Guard, e.Post} {
			if err := core.BindersApartFrom(c, global(fieldNames(e.Event), nil)); err != nil {
				return fmt.Errorf("line %d: %s --> %s : %s: %w", r.edgeAt[i], e.Src, e.Dst, e.Event.Label(), err)
			}
		}
	}
	for _, s := range r.p.Syncs {
		var ids []string
		for _, t := range s.Targets {
			ids = append(ids, t.ID)
		}
		if err := core.BindersApartFrom(s.Guard, global(nil, ids)); err != nil {
			return fmt.Errorf("sync %s: %w", s.Event, err)
		}
	}
	for _, c := range r.p.Constrains {
		if err := core.BindersApartFrom(c.Guard, global(fieldNames(c.Event), nil)); err != nil {
			return fmt.Errorf("constrain %s: %w", c.Event.Label(), err)
		}
	}
	return nil
}

// fieldNames are the names of the fields of e, in order.
func fieldNames(e core.Event) []string {
	names := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		names[i] = f.Name
	}
	return names
}
