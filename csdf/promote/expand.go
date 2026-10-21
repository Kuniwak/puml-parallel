package promote

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// LoadFunc resolves the path written in an !include into the local diagram it
// names. The caller decides what the path is relative to.
type LoadFunc func(path string) (*csdf.Diagram, error)

// Severity is how much a diagnostic matters. An error leaves the diagram
// unprinted, so an unsound expansion never reaches the tools downstream.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

// Diagnostic is one thing the expansion has to say about the diagram it read.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Line     int      `json:"line"` // 1-based source line, 0 when there is none.
	Message  string   `json:"message"`
}

func (d Diagnostic) String() string {
	if d.Line == 0 {
		return fmt.Sprintf("%s: %s", d.Severity, d.Message)
	}
	return fmt.Sprintf("%s: line %d: %s", d.Severity, d.Line, d.Message)
}

// Expansion is the expanded diagram together with the origin of every edge the
// expansion generated.
type Expansion struct {
	Diagram *csdf.Diagram `json:"diagram"`
	// Edges is Diagram.Edges again, each with where it came from. The two are
	// kept in step by sortEdges, which is the only thing that reorders them.
	Edges []ExpandedEdge `json:"edges"`
	// Promotes are the <<promote>> blocks the expansion came from, in source
	// order, and Locals the local diagrams they include, by the path the
	// block wrote. A program reading the expansion needs them: a generated
	// clause names a local state and a local clause reads local variables
	// that the expansion's text never spells out.
	Promotes []Promote                `json:"promotes"`
	Locals   map[string]*csdf.Diagram `json:"locals"`
	// Start is the start edge's post, clause by clause: what the author wrote,
	// then that every map the start state holds is empty, which the
	// <<promote>> blocks say. Diagram.StartEdge.Post spells the author's
	// clauses alone (see conjoin).
	Start []ExpandedClause `json:"start"`
}

// ExpandedEdge is one edge of the expansion and the local edge it came from.
// Origin is empty for an edge the author wrote by hand, and so are the clause
// lists: Guard and Post are the clauses of a generated edge one by one, in the
// order the predicate joins them, each saying what it is.
type ExpandedEdge struct {
	Edge   csdf.Edge        `json:"edge"`
	Origin string           `json:"origin,omitempty"`
	Guard  []ExpandedClause `json:"guard,omitempty"`
	Post   []ExpandedClause `json:"post,omitempty"`
}

// ClauseKind says what a clause of a generated edge is. The generated kinds
// have a meaning a tool can act on without reading the wording; a local clause
// and a constrain clause are the author's opaque text.
type ClauseKind string

const (
	ClauseInDom     ClauseKind = "in_dom"     // the instance exists: ID ∈ dom Map
	ClauseNotInDom  ClauseKind = "not_in_dom" // the instance does not exist yet
	ClauseAtState   ClauseKind = "at_state"   // the instance is in local state State
	ClauseUpdate    ClauseKind = "update"     // the instance moves to local state State
	ClauseCreate    ClauseKind = "create"     // the instance is created in local state State
	ClauseDelete    ClauseKind = "delete"     // the instance is removed from Map
	ClauseFrame     ClauseKind = "frame"      // state variable Var is unchanged
	ClauseEmpty     ClauseKind = "empty"      // map Map starts empty, on the start edge
	ClauseLocal     ClauseKind = "local"      // a clause of the local edge, verbatim
	ClauseConstrain ClauseKind = "constrain"  // a clause of a constrain directive, verbatim
	// ClauseDropped is a guard clause that read a map Restrict left out. It is
	// listed rather than lost, and taken as true: it is not in the predicate.
	ClauseDropped ClauseKind = "dropped"
)

// ExpandedClause is one clause of a generated edge. Text is the clause as it
// stands in the predicate; the other fields are what the kind needs: Map and ID
// for the clauses about an instance, State for the local state they name, Var
// for a frame, and Path and Line for where a local or constrain clause was
// written (a constrain has no Path: it is in the global diagram).
type ExpandedClause struct {
	Kind ClauseKind `json:"kind"`
	// Text is the clause as its language reads it, its escapes removed; an
	// opaque clause has none.
	Text string `json:"text,omitempty"`
	// Opaque marks the tool word @opaque, a clause not formalised yet.
	Opaque bool `json:"opaque,omitempty"`
	// Rename maps a name Text reads to the name of the predicate it stands
	// for, as the clause's renaming @[from := to] says.
	Rename map[string]string `json:"rename,omitempty"`
	// Eq names the two values the tool word @eq(a, b) says are equal; such a
	// clause has no text.
	Eq []string `json:"eq,omitempty"`
	// Formal is the clause's formal twin when the author wrote one next to it
	// (a constrain's "formal:" line); a local clause's twin lives with the
	// local diagram and is looked up by Text.
	Formal string `json:"formal,omitempty"`
	Map    string `json:"map,omitempty"`
	ID     string `json:"id,omitempty"`
	State  string `json:"state,omitempty"`
	Var    string `json:"var,omitempty"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	// Maps are the left-out maps a dropped clause read.
	Maps []string `json:"maps,omitempty"`
}

// authored is a clause the author wrote, of the given kind, with what says
// where it came from: its text as its language reads it, or opaque.
// A renaming that does not parse is an error of the expansion, at the line the
// clause was written on: a diagram built in code need not have been checked.
func (e *expander) authored(kind ClauseKind, clause csdf.Predicate, from ExpandedClause) ExpandedClause {
	from.Kind = kind
	if csdf.IsOpaque(clause) {
		from.Opaque = true
		return from
	}
	if a, b, ok, err := csdf.SplitEquality(clause); err != nil {
		e.errorf(from.Line, "%v", err)
		return from
	} else if ok {
		from.Eq = []string{a, b}
		return from
	}
	renames, body, err := csdf.SplitRenaming(clause)
	if err != nil {
		e.errorf(from.Line, "%v", err)
	}
	from.Text = csdf.ClauseText(body)
	from.Rename = csdf.RenamesMap(renames)
	return from
}

// conjoin is the predicate the clauses spell, joined by the connective. A
// dropped clause is listed beside the predicate, not in it, and so is an
// empty one: the start edge's text stays the author's, so a tool reading the
// expansion as a plain diagram reads what it did before maps started empty.
func conjoin(clauses []ExpandedClause) csdf.Predicate {
	texts := make([]csdf.Predicate, 0, len(clauses))
	for _, c := range clauses {
		switch {
		case c.Kind == ClauseDropped || c.Kind == ClauseEmpty:
		case c.Opaque:
			texts = append(texts, csdf.PredicateOpaque)
		case len(c.Eq) == 2:
			texts = append(texts, csdf.NewEquality(c.Eq[0], c.Eq[1]))
		default:
			// An escaped text holds no renaming of its own, so this cannot fail.
			clause, err := csdf.RenameClause(csdf.EscapeClause(c.Text), csdf.RenamesFromMap(c.Rename))
			if err != nil {
				panic(fmt.Sprintf("promote.conjoin: %v", err))
			}
			texts = append(texts, clause)
		}
	}
	return csdf.ConjoinClauses(texts)
}

// Expand turns every promotion into edges on the state its block was written in.
//
// The wording of the generated clauses is templates; pass DefaultTemplates for
// the symbolic one. It returns no expansion when a diagnostic is an error: an
// unsound expansion must not reach the tools downstream.
func Expand(g *GlobalDiagram, load LoadFunc, templates *Templates) (*Expansion, []Diagnostic) {
	e := &expander{
		global:    g,
		load:      load,
		templates: templates,
		locals:    map[csdf.Var]*csdf.Diagram{},
		paths:     map[csdf.Var]string{},
		owned:     map[ownership]bool{},
		diags:     append([]Diagnostic{}, g.Diagnostics...),
	}
	return e.run()
}

type expander struct {
	global    *GlobalDiagram
	load      LoadFunc
	templates *Templates

	locals map[csdf.Var]*csdf.Diagram
	// paths is where each map's local diagram was read from. A block with no
	// body has no path of its own, but its edges came from the same file.
	paths map[csdf.Var]string
	// order is the block that carries each map's !include, in source order, so
	// the checks and the expansion read the maps in the order they were written.
	order []Promote
	// plans is one worked-out sync per directive, and owned marks the events a
	// sync took over so that the maps do not expand them on their own.
	plans []syncPlan
	owned map[ownership]bool
	diags []Diagnostic
}

func (e *expander) run() (*Expansion, []Diagnostic) {
	e.check()
	if HasError(e.diags) {
		return nil, e.sorted()
	}

	out := e.global.Core.Clone()
	edges := make([]ExpandedEdge, 0, len(out.Edges))
	for _, edge := range out.Edges {
		edges = append(edges, ExpandedEdge{Edge: edge})
	}

	for _, p := range e.global.Promotes {
		for _, edge := range e.locals[p.Map].Edges {
			if name := edge.Event.Channel(); e.owned[ownership{p.In, p.Map, name}] {
				continue
			}
			edges = append(edges, e.promoteEdge(p, edge))
		}
	}

	edges = e.mergeSyncs(edges)
	e.constrain(edges)
	if HasError(e.diags) {
		return nil, e.sorted()
	}

	locals := make(map[string]*csdf.Diagram, len(e.locals))
	for m, d := range e.locals {
		locals[e.paths[m]] = d
	}
	x := &Expansion{Diagram: out, Edges: edges, Promotes: append([]Promote{}, e.global.Promotes...), Locals: locals, Start: e.start(out)}
	x.sortEdges()
	return x, e.sorted()
}

// start is the start edge's post: the author's clauses, then one empty clause
// per map the start state holds, in the order the maps were promoted.
func (e *expander) start(d *csdf.Diagram) []ExpandedClause {
	out := []ExpandedClause{}
	for _, c := range d.StartEdge.Post.Clauses() {
		out = append(out, e.authored(ClauseLocal, c, ExpandedClause{Line: d.StartEdge.Line}))
	}
	held := map[csdf.Var]bool{}
	for _, v := range d.States[d.StartEdge.Dst].Vars {
		held[v.Name] = true
	}
	for _, p := range e.order {
		if held[p.Map] {
			out = append(out, ExpandedClause{Kind: ClauseEmpty, Text: e.templates.clause(clauseEmpty, Clause{Map: string(p.Map)}), Map: string(p.Map)})
		}
	}
	return out
}

// edgeParts is one local edge promoted through one map: the clauses it
// contributes to a global edge, and what it needs to name itself.
type edgeParts struct {
	guard []ExpandedClause
	post  []ExpandedClause
	name  string       // The event's channel.
	args  []csdf.Field // The event's own fields, after the instance ID.
	id    string       // The parameter the instance ID is written as.
	tau   bool
	src   string // The local source state's name, for the origin comment.
	dst   string // The local destination state's name.
}

// parts promotes one local edge through map p, writing the instance ID as id.
// The clauses are not joined here: a synced edge is the conjunction of the parts
// of one local edge per map, and a plain one is the conjunction of just this.
func (e *expander) parts(p Promote, edge csdf.Edge, id string) edgeParts {
	local := e.locals[p.Map]
	absent := local.StartEdge.Dst
	src, dst := local.States[edge.Src], local.States[edge.Dst]

	parts := edgeParts{src: src.Name, dst: dst.Name, id: id, tau: edge.Event == csdf.Tau}
	parts.name, parts.args = edge.Event.Channel(), edge.Event.Fields()

	c := Clause{Map: string(p.Map), ID: id, Src: src.Name, Dst: dst.Name}
	about := func(kind ClauseKind, name string, state string) ExpandedClause {
		return ExpandedClause{Kind: kind, Text: e.templates.clause(name, c), Map: string(p.Map), ID: id, State: state}
	}
	switch {
	case edge.Src == absent: // Creation: the instance starts to exist.
		parts.guard = append(parts.guard, about(ClauseNotInDom, clauseNotInDom, ""))
		parts.post = append(parts.post, about(ClauseCreate, clauseCreate, dst.Name))

	case edge.Dst == absent: // Deletion: the instance stops existing.
		parts.guard = append(parts.guard, about(ClauseInDom, clauseInDom, ""), about(ClauseAtState, clauseAtState, src.Name))
		parts.post = append(parts.post, about(ClauseDelete, clauseDelete, ""))

	default:
		parts.guard = append(parts.guard, about(ClauseInDom, clauseInDom, ""), about(ClauseAtState, clauseAtState, src.Name))
		parts.post = append(parts.post, about(ClauseUpdate, clauseUpdate, dst.Name))
	}

	parts.guard = append(parts.guard, e.localClauses(p, edge, edge.Guard)...)
	// The post of a deletion says something about an instance that is gone, so
	// it is dropped rather than carried into the expansion.
	if edge.Dst != absent {
		parts.post = append(parts.post, e.localClauses(p, edge, edge.Post)...)
	}
	return parts
}

// localClauses copies the clauses of a local predicate verbatim, each saying
// which local edge it came from.
func (e *expander) localClauses(p Promote, edge csdf.Edge, pred csdf.Predicate) []ExpandedClause {
	var clauses []ExpandedClause
	for _, c := range pred.Clauses() {
		clauses = append(clauses, e.authored(ClauseLocal, c, ExpandedClause{Map: string(p.Map), Path: e.paths[p.Map], Line: edge.Line}))
	}
	return clauses
}

// promoteEdge lifts one local edge onto the state its block was written in.
func (e *expander) promoteEdge(p Promote, edge csdf.Edge) ExpandedEdge {
	parts := e.parts(p, edge, p.IDParam)
	out := e.compose(p.In, []edgeParts{parts}, p.Map)
	out.Origin = fmt.Sprintf("promote: %s 〈%s〉 → 〈%s〉", e.paths[p.Map], parts.src, parts.dst)
	return out
}

// compose joins the parts of one or more local edges into one global self-loop
// on state g, framing every state variable of g but the maps the parts moved.
func (e *expander) compose(g csdf.StateID, parts []edgeParts, moved ...csdf.Var) ExpandedEdge {
	var guard, post []ExpandedClause
	var ids []string
	var args []csdf.Field
	tau := false

	for _, part := range parts {
		guard = append(guard, part.guard...)
		post = append(post, part.post...)
		tau = tau || part.tau
		if !part.tau {
			ids = appendUnique(ids, part.id)
			args = appendUnique(args, part.args...)
		}
	}
	post = append(post, e.frame(g, moved)...)

	event := csdf.Tau
	if !tau {
		fields := make([]csdf.Field, 0, len(ids)+len(args))
		for _, id := range ids {
			fields = append(fields, csdf.Field{Dir: csdf.Received, Name: id})
		}
		event = csdf.NewEvent(parts[0].name, append(fields, args...)...)
	}

	return ExpandedEdge{
		Edge: csdf.Edge{
			Src:   g,
			Dst:   g,
			Event: event,
			Guard: conjoin(guard),
			Post:  conjoin(post),
		},
		Guard: guard,
		Post:  post,
	}
}

// appendUnique appends the values that are not in dst yet. Arguments of one name
// mean one thing, which is how a synced event ends up with one of each.
func appendUnique[T comparable](dst []T, values ...T) []T {
	for _, v := range values {
		if !slices.Contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

// frame says that the state variables this edge does not touch are unchanged.
// The maps it moved are not framed: ⊕, ∪ and ⩤ already say what happens to every
// key but the one the edge is about.
func (e *expander) frame(g csdf.StateID, moved []csdf.Var) []ExpandedClause {
	var clauses []ExpandedClause
	for _, v := range e.global.Core.States[g].Vars {
		if slices.Contains(moved, v.Name) {
			continue
		}
		clauses = append(clauses, ExpandedClause{Kind: ClauseFrame, Text: e.templates.clause(clauseFrame, Clause{Var: string(v.Name)}), Var: string(v.Name)})
	}
	return clauses
}

// sortEdges puts the edges in the canonical order, carrying each origin comment
// along with the edge it belongs to.
func (x *Expansion) sortEdges() {
	slices.SortStableFunc(x.Edges, func(a, b ExpandedEdge) int {
		return csdf.CompareEdge(a.Edge, b.Edge)
	})

	x.Diagram.Edges = make([]csdf.Edge, len(x.Edges))
	for i, edge := range x.Edges {
		x.Diagram.Edges[i] = edge.Edge
	}
}

// MapLoader resolves !include paths from an in-memory table of sources, which is
// what a test - or anything holding the local diagrams already - wants.
func MapLoader(sources map[string]string) LoadFunc {
	return func(path string) (*csdf.Diagram, error) {
		source, ok := sources[path]
		if !ok {
			return nil, fmt.Errorf("no such file: %q", path)
		}
		return csdf.Parse(source)
	}
}

// FileLoader resolves !include paths against base, the directory the global
// diagram was read from.
func FileLoader(base string) LoadFunc {
	return func(path string) (*csdf.Diagram, error) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		return csdf.LoadDiagram(path)
	}
}
