package obligationir

import (
	"cmp"
	"hash"
	"hash/crc32"
	"slices"
	"strconv"

	"github.com/Kuniwak/puml-parallel/csdf"
)

type IRPredicateID uint32

func ComparePredicateID(a, b IRPredicateID) int {
	return cmp.Compare(a, b)
}

// FormatPredicateID renders an ID as the base-36 suffix of a generated predicate
// name (pred_<id>). Every backend must name a given predicate identically, so this
// is the single definition of that spelling.
func FormatPredicateID(id IRPredicateID) string {
	return strconv.FormatUint(uint64(id), 36)
}

// IRLivelockFree is a prover-agnostic intermediate representation of the proof
// obligation that the diagram is livelock free, i.e. no reachable state admits an
// infinite run of internal (τ) transitions. Natural-language Guard/Post predicates
// are left opaque, deduplicated into Predicates under the hash of their text and
// argument types; an edge names one by id. A downstream generator expands this IR
// into Lean or Isabelle, and the predicate bodies are supplied separately. Both
// are out of scope here.
type IRLivelockFree struct {
	// Structurally is true when no reachable τ-only cycle exists, in which case
	// the obligation holds regardless of the predicates.
	Structurally bool                          `json:"structurally"`
	Predicates   map[IRPredicateID]IRPredicate `json:"predicates"`
	States       map[csdf.StateID]IRState      `json:"states"`    // the state space as an ADT
	Constants    []IRConst                     `json:"constants"` // global opaque constants in scope
	Edges        []IREdge                      `json:"edges"`     // the labelled transitions
	Init         IRInit                        `json:"init"`
}

type IRState struct {
	Fields []IRField `json:"fields"`
	Line   int       `json:"line"` // 1-based
}

type IRStateWithID struct {
	StateID csdf.StateID
	Fields  []IRField
	Line    int // 1-based
}

func SortIRStates(m map[csdf.StateID]IRState) []IRStateWithID {
	linesMap := make(map[int]IRStateWithID, len(m))
	lines := make([]int, 0, len(m))
	for id, s := range m {
		linesMap[s.Line] = IRStateWithID{
			StateID: id,
			Fields:  s.Fields,
			Line:    s.Line,
		}
		lines = append(lines, s.Line)
	}
	slices.Sort(lines)

	res := make([]IRStateWithID, 0, len(m))
	for _, line := range lines {
		res = append(res, linesMap[line])
	}
	return res
}

type IRField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// IRConst is a global opaque constant a predicate body may reference.
type IRConst struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// IRArg is one argument of a predicate (an event parameter or a state variable).
// Primed marks a post-state variable.
type IRArg struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Primed bool   `json:"primed"`
}

// IRPredicate is an opaque predicate symbol with its argument signature and the
// verbatim natural-language text it stands for. Kind is "guard", "post", or "init".
type IRPredicate struct {
	Args []IRArg        `json:"args"`
	Text csdf.Predicate `json:"text"`
}

func (p IRPredicate) Hash(h hash.Hash32) IRPredicateID {
	h.Write([]byte(p.Text))
	for _, arg := range p.Args {
		h.Write([]byte{0x00})
		h.Write([]byte(arg.Type))
	}
	res := IRPredicateID(h.Sum32())
	h.Reset()
	return res
}

type IRPredicateWithID struct {
	ID        IRPredicateID `json:"hash"`
	Predicate IRPredicate   `json:"predicate"`
}

func (p IRPredicate) WithID(h hash.Hash32) IRPredicateWithID {
	return IRPredicateWithID{
		ID:        p.Hash(h),
		Predicate: p,
	}
}

func ComparePredicateWithID(a, b IRPredicateWithID) int {
	return ComparePredicateID(a.ID, b.ID)
}

func IRPredicatesWithHash(ps []IRPredicate) []IRPredicateWithID {
	h := crc32.NewIEEE()
	res := make([]IRPredicateWithID, len(ps))
	for i, p := range ps {
		res[i] = p.WithID(h)
	}
	return res
}

// IREdge is one transition. Guard/Post name a shared predicate symbol, while
// GuardArgs/PostArgs are this occurrence's own arguments: the variables the
// source and target states of *this* edge bind. The two cannot be read off the
// shared predicate, whose recorded argument names belong to whichever occurrence
// registered it, so applying a shared symbol to those names would capture or
// leave free the wrong variables.
type IREdge struct {
	Src         csdf.StateID  `json:"src"`
	Dst         csdf.StateID  `json:"dst"`
	Event       csdf.Event    `json:"event"`
	EventParams []IRArg       `json:"event_params"`
	Guard       IRPredicateID `json:"guard"`
	GuardArgs   []IRArg       `json:"guard_args"`
	Post        IRPredicateID `json:"post"`
	PostArgs    []IRArg       `json:"post_args"`
	Line        int           `json:"line"` // 1-based
}

// IRInit names the start state. PostArgs is this occurrence's own argument list;
// see IREdge.
type IRInit struct {
	Dst      csdf.StateID  `json:"state"`
	Post     IRPredicateID `json:"post"`
	PostArgs []IRArg       `json:"post_args"`
	Line     int           `json:"line"` // 1-based
}

// PredicateSet accumulates the opaque predicates of one or more diagrams,
// deduplicating them under the hash of their text and argument types. Sharing one
// set across diagrams is what makes an identical predicate collapse to a single
// pred_<id> placeholder in a multi-diagram obligation.
type PredicateSet struct {
	h hash.Hash32
	m map[IRPredicateID]IRPredicate
}

func NewPredicateSet(capacity int) *PredicateSet {
	return &PredicateSet{
		h: crc32.NewIEEE(),
		m: make(map[IRPredicateID]IRPredicate, capacity),
	}
}

// Add records p and returns the id every backend names it by.
func (ps *PredicateSet) Add(p IRPredicate) IRPredicateID {
	id := p.Hash(ps.h)
	ps.m[id] = p
	return id
}

// Map returns the accumulated predicates keyed by id.
func (ps *PredicateSet) Map() map[IRPredicateID]IRPredicate { return ps.m }

// BuildStates converts the diagram's states into the IR state space.
func BuildStates(d *csdf.Diagram) map[csdf.StateID]IRState {
	states := make(map[csdf.StateID]IRState, len(d.States))
	for _, st := range csdf.SortedStates(d.States) {
		fields := make([]IRField, 0, len(st.Vars))
		for _, v := range st.Vars {
			fields = append(fields, IRField{Name: string(v.Name), Type: v.Type})
		}
		states[st.ID] = IRState{
			Fields: fields,
			Line:   st.Line,
		}
	}
	return states
}

// BuildInit converts the diagram's start edge into the IR init, registering its
// post predicate (which constrains the start state's variables) in ps.
func BuildInit(ps *PredicateSet, d *csdf.Diagram, states map[csdf.StateID]IRState) IRInit {
	start := states[d.StartEdge.Dst]
	args := make([]IRArg, 0, len(start.Fields))
	for _, f := range start.Fields {
		args = append(args, IRArg{
			Name: f.Name,
			Type: f.Type,
		})
	}
	return IRInit{
		Dst:      d.StartEdge.Dst,
		Post:     ps.Add(IRPredicate{Args: args, Text: d.StartEdge.Post}),
		PostArgs: args,
		Line:     d.StartEdge.Line,
	}
}

// BuildEnd converts the diagram's end edge into the IR end, registering its
// guard predicate in ps. It returns nil when the diagram cannot terminate.
func BuildEnd(ps *PredicateSet, d *csdf.Diagram, states map[csdf.StateID]IRState) *IREnd {
	if d.EndEdge == nil {
		return nil
	}

	src := states[d.EndEdge.Src]
	args := make([]IRArg, 0, len(src.Fields))
	for _, f := range src.Fields {
		args = append(args, IRArg{
			Name: f.Name,
			Type: f.Type,
		})
	}
	return &IREnd{
		Src:       d.EndEdge.Src,
		Guard:     ps.Add(IRPredicate{Args: args, Text: d.EndEdge.Guard}),
		GuardArgs: args,
		Line:      d.EndEdge.Line,
	}
}

// BuildEdges converts the diagram's transitions into IR edges, registering each
// guard and post predicate in ps.
func BuildEdges(ps *PredicateSet, d *csdf.Diagram, states map[csdf.StateID]IRState) []IREdge {
	edges := make([]IREdge, 0, len(d.Edges))
	for _, e := range d.Edges {
		var evParams []csdf.StateVar // TODO
		preVars := states[e.Src]
		postVars := states[e.Dst]

		guardArgs := make([]IRArg, 0, len(evParams)+len(preVars.Fields))
		for _, evParam := range evParams {
			guardArgs = append(guardArgs, IRArg{
				Name: string(evParam.Name),
				Type: evParam.Type,
			})
		}
		for _, preVar := range preVars.Fields {
			guardArgs = append(guardArgs, IRArg{
				Name: preVar.Name,
				Type: preVar.Type,
			})
		}
		guardID := ps.Add(IRPredicate{Args: guardArgs, Text: e.Guard})

		postArgs := make([]IRArg, 0, len(guardArgs)+len(postVars.Fields))
		postArgs = append(postArgs, guardArgs...)
		for _, postVar := range postVars.Fields {
			postArgs = append(postArgs, IRArg{
				Name:   postVar.Name,
				Type:   postVar.Type,
				Primed: true,
			})
		}
		postID := ps.Add(IRPredicate{Args: postArgs, Text: e.Post})

		edges = append(edges, IREdge{
			Src:         e.Src,
			Dst:         e.Dst,
			Event:       e.Event,
			EventParams: []IRArg{},
			Guard:       guardID,
			GuardArgs:   guardArgs,
			Post:        postID,
			PostArgs:    postArgs,
			Line:        e.Line,
		})
	}
	return edges
}

// BuildLivelockFree builds the livelock-freedom proof obligation IR for d. The
// structural τ-cycle check (CheckLivelockFree) is used only to set
// StructurallyLivelockFree; the obligation itself is the global property and does
// not depend on a particular witness.
func BuildLivelockFree(d *csdf.Diagram) IRLivelockFree {
	_, free := csdf.CheckLivelockFree(d)

	ps := NewPredicateSet(len(d.Edges)*2 + 1)
	states := BuildStates(d)

	init := BuildInit(ps, d, states)
	edges := BuildEdges(ps, d, states)

	return IRLivelockFree{
		Structurally: free,
		States:       states,
		Constants:    []IRConst{},
		Init:         init,
		Edges:        edges,
		Predicates:   ps.Map(),
	}
}

func TauEdges(es []IREdge) []IREdge {
	res := make([]IREdge, 0, len(es))
	for _, e := range es {
		if e.Event == csdf.Tau {
			res = append(res, e)
		}
	}
	return res
}

func Predicates(ps map[IRPredicateID]IRPredicate) []IRPredicateWithID {
	hs := make([]IRPredicateWithID, 0, len(ps))
	for id, p := range ps {
		hs = append(hs, IRPredicateWithID{
			ID:        id,
			Predicate: p,
		})
	}
	slices.SortFunc(hs, ComparePredicateWithID)
	return hs
}

// HasVars reports whether any state has a variable, in which case the json datatype is
// emitted (otherwise it would be unused).
func HasVars(states map[csdf.StateID]IRState) bool {
	for _, st := range states {
		if len(st.Fields) > 0 {
			return true
		}
	}
	return false
}
