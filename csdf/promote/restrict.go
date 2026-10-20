package promote

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Restrict keeps the families named by keep - each a map or a type - and
// leaves out the others: their maps, and the edges only they move.
//
// On an edge a kept family takes together with a left-out one, the kept side
// stays and the event keeps every argument, so the left-out side becomes the
// environment, which offers the event. A guard clause that read a left-out
// map is not lost: it becomes a ClauseDropped, outside the predicate, and is
// reported as a warning. With every such clause taken as true, each run of the
// full expansion projects onto a run of the restriction, so a broken invariant
// or an event-avoiding cycle over the kept families survives it; a deadlock
// that a left-out family caused by refusing does not.
//
// A constrain clause is known to read a map only through its formal twin. One
// with no twin stays and is reported once, since nothing says whether it reads
// a left-out map. Hand-written edges are kept as they were, with a warning
// when they touch a state that held a left-out map. What a formal twin reads
// is the names it spells; a twin that reaches a map through a helper defeats
// that. x is not changed.
func Restrict(x *Expansion, keep []string) (*Expansion, []Diagnostic) {
	r := &restriction{}
	kept, dropped := r.partition(x.Promotes, keep)
	if HasError(r.diags) {
		return nil, r.diags
	}

	out := &Expansion{Diagram: x.Diagram.Clone(), Locals: map[string]*csdf.Diagram{}}
	for id, s := range out.Diagram.States {
		s.Vars = slices.DeleteFunc(s.Vars, func(v csdf.StateVar) bool { return dropped[v.Name] })
		out.Diagram.States[id] = s
	}
	for _, p := range x.Promotes {
		if kept[p.Map] {
			out.Promotes = append(out.Promotes, p)
			if p.Path != "" {
				out.Locals[p.Path] = x.Locals[p.Path]
			}
		}
	}

	for _, e := range x.Edges {
		if e.Origin == "" {
			r.handWritten(e, x.Diagram, dropped)
			out.Edges = append(out.Edges, e)
			continue
		}
		if !movesAny(e, kept) {
			continue
		}
		out.Edges = append(out.Edges, r.edge(e, dropped))
	}
	out.Start = r.start(x, dropped)
	out.sortEdges()
	return out, r.diags
}

// handWritten warns of a hand-written edge between states that held a
// left-out map: its predicates are opaque text, which may still name the map.
func (r *restriction) handWritten(e ExpandedEdge, d *csdf.Diagram, dropped map[csdf.Var]bool) {
	var held []string
	for _, id := range []csdf.StateID{e.Edge.Src, e.Edge.Dst} {
		for _, v := range d.States[id].Vars {
			if dropped[v.Name] && !slices.Contains(held, string(v.Name)) {
				held = append(held, string(v.Name))
			}
		}
	}
	if len(held) == 0 {
		return
	}
	r.once(fmt.Sprintf("hand\x00%d", e.Edge.Line), Diagnostic{
		Severity: SeverityWarning, Line: e.Edge.Line,
		Message: fmt.Sprintf("the hand-written edge %s --> %s : %s is kept as written, and may still read %s, which -only leaves out", e.Edge.Src, e.Edge.Dst, e.Edge.Event, strings.Join(held, ", ")),
	})
}

// start keeps the empty clauses of the kept maps and the author's clauses as
// written; like a hand-written edge, those may still read a left-out map the
// start state held, which is said.
func (r *restriction) start(x *Expansion, dropped map[csdf.Var]bool) []ExpandedClause {
	out := []ExpandedClause{}
	authored := false
	for _, c := range x.Start {
		if c.Kind == ClauseEmpty && dropped[csdf.Var(c.Map)] {
			continue
		}
		authored = authored || c.Kind == ClauseLocal
		out = append(out, c)
	}
	var held []string
	for _, v := range x.Diagram.States[x.Diagram.StartEdge.Dst].Vars {
		if dropped[v.Name] {
			held = append(held, string(v.Name))
		}
	}
	if authored && len(held) > 0 {
		r.diags = append(r.diags, Diagnostic{
			Severity: SeverityWarning, Line: x.Diagram.StartEdge.Line,
			Message: fmt.Sprintf("the start edge's clauses are kept as written, and may still read %s, which -only leaves out", strings.Join(held, ", ")),
		})
	}
	return out
}

type restriction struct {
	diags []Diagnostic
	// reported marks what was said already: a dropped clause is generated onto
	// many edges, and a constrain matches many, but each is one thing.
	reported map[string]bool
}

// partition resolves the names in keep against the blocks: the maps kept,
// and the maps left out.
func (r *restriction) partition(promotes []Promote, keep []string) (map[csdf.Var]bool, map[csdf.Var]bool) {
	kept, dropped := map[csdf.Var]bool{}, map[csdf.Var]bool{}
	for _, name := range keep {
		found := false
		for _, p := range promotes {
			if string(p.Map) == name || p.Type == name {
				kept[p.Map] = true
				found = true
			}
		}
		if !found {
			r.diags = append(r.diags, Diagnostic{Severity: SeverityError, Message: fmt.Sprintf("-only names %q, which is neither a promoted map nor the type of one", name)})
		}
	}
	for _, p := range promotes {
		if !kept[p.Map] {
			dropped[p.Map] = true
		}
	}
	return kept, dropped
}

// movesAny reports whether a generated edge moves an instance of a kept map.
func movesAny(e ExpandedEdge, kept map[csdf.Var]bool) bool {
	return slices.ContainsFunc(e.Post, func(c ExpandedClause) bool {
		return isMove(c.Kind) && kept[csdf.Var(c.Map)]
	})
}

func isMove(k ClauseKind) bool {
	return k == ClauseUpdate || k == ClauseCreate || k == ClauseDelete
}

// edge is a generated edge without what the left-out maps contributed: their
// guard clauses become dropped ones, and their post clauses and frames go,
// since they only say what happens to a map that is not there.
func (r *restriction) edge(e ExpandedEdge, dropped map[csdf.Var]bool) ExpandedEdge {
	guard := make([]ExpandedClause, 0, len(e.Guard))
	for _, c := range e.Guard {
		guard = append(guard, r.guardClause(c, dropped))
	}
	post := slices.DeleteFunc(slices.Clone(e.Post), func(c ExpandedClause) bool {
		return dropped[csdf.Var(c.Map)] || dropped[csdf.Var(c.Var)]
	})

	e.Guard, e.Post = guard, post
	e.Edge.Guard, e.Edge.Post = conjoin(guard), conjoin(post)
	return e
}

func (r *restriction) guardClause(c ExpandedClause, dropped map[csdf.Var]bool) ExpandedClause {
	var reads []string
	switch {
	case c.Opaque:
		return c // It says nothing a tool can read, so there is nothing to drop.
	case c.Kind == ClauseConstrain && c.Formal == "":
		r.once(fmt.Sprintf("unknown\x00%d\x00%s", c.Line, c.Text), Diagnostic{
			Severity: SeverityWarning, Line: c.Line,
			Message: fmt.Sprintf("the constrain clause %q has no formal twin, so -only cannot tell whether it reads a map it leaves out; it is kept", c.Text),
		})
		return c
	case c.Kind == ClauseConstrain:
		reads = droppedNames(c.Formal, dropped)
	case dropped[csdf.Var(c.Map)]:
		reads = []string{c.Map}
	}
	if len(reads) == 0 {
		return c
	}

	where := fmt.Sprintf("line %d", c.Line)
	if c.Path != "" {
		where = fmt.Sprintf("%s:%d", c.Path, c.Line)
	}
	line := c.Line
	if c.Path != "" || c.Kind != ClauseConstrain {
		line = 0 // the line is not one of the global diagram's
	}
	r.once(fmt.Sprintf("dropped\x00%s\x00%s", where, c.Text), Diagnostic{
		Severity: SeverityWarning, Line: line,
		Message: fmt.Sprintf("the guard clause %q reads %s, which -only leaves out; it is taken as true", c.Text, strings.Join(reads, ", ")),
	})
	return ExpandedClause{Kind: ClauseDropped, Text: c.Text, Formal: c.Formal, Map: c.Map, ID: c.ID, State: c.State, Path: c.Path, Line: c.Line, Maps: reads}
}

func (r *restriction) once(key string, d Diagnostic) {
	if r.reported == nil {
		r.reported = map[string]bool{}
	}
	if r.reported[key] {
		return
	}
	r.reported[key] = true
	r.diags = append(r.diags, d)
}

// A formal twin is a Python expression, so the names it reads are its
// identifiers outside its string literals: that is grammar, not a guess at
// prose.
var (
	pyStringRe = regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"`)
	pyIdentRe  = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)
)

// droppedNames returns the left-out maps a formal twin reads, in the order
// it first reads them.
func droppedNames(formal string, dropped map[csdf.Var]bool) []string {
	var out []string
	for _, name := range pyIdentRe.FindAllString(pyStringRe.ReplaceAllString(formal, `""`), -1) {
		if dropped[csdf.Var(name)] && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}
