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

func (ir IRLivelockFree) UsedMap() map[IRPredicateID]struct{} {
	m := make(map[IRPredicateID]struct{}, len(ir.Edges)*2)
	for _, e := range ir.Edges {
		if e.Event != csdf.Tau {
			continue
		}

		m[e.Guard] = struct{}{}
		m[e.Post] = struct{}{}
	}
	return m
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

// IREdge is one transition. Guard/Post hold either a predicate symbol or the
// literal "True" when the predicate is omitted.
type IREdge struct {
	Src         csdf.StateID  `json:"src"`
	Dst         csdf.StateID  `json:"dst"`
	Event       csdf.Event    `json:"event"`
	EventParams []IRArg       `json:"event_params"`
	Guard       IRPredicateID `json:"guard"`
	Post        IRPredicateID `json:"post"`
	Line        int           `json:"line"` // 1-based
}

// IRInit names the start state.
type IRInit struct {
	Dst  csdf.StateID  `json:"state"`
	Post IRPredicateID `json:"post"`
	Line int           `json:"line"` // 1-based
}

// BuildLivelockFree builds the livelock-freedom proof obligation IR for d. The
// structural τ-cycle check (CheckLivelockFree) is used only to set
// StructurallyLivelockFree; the obligation itself is the global property and does
// not depend on a particular witness.
func BuildLivelockFree(d *csdf.Diagram) IRLivelockFree {
	_, free := csdf.CheckLivelockFree(d)

	ir := IRLivelockFree{
		Structurally: free,
		States:       make(map[csdf.StateID]IRState, len(d.States)),
		Predicates:   make(map[IRPredicateID]IRPredicate, len(d.Edges)*2+1),
		Constants:    []IRConst{},
		Edges:        make([]IREdge, 0, len(d.Edges)),
	}

	ss := csdf.SortedStates(d.States)
	for _, st := range ss {
		fields := make([]IRField, 0, len(st.Vars))
		for _, v := range st.Vars {
			fields = append(fields, IRField{Name: string(v.Name), Type: v.Type})
		}
		ir.States[st.ID] = IRState{
			Fields: fields,
			Line:   st.Line,
		}
	}

	h := crc32.NewIEEE()

	initPostVars := ir.States[d.StartEdge.Dst]
	initArgs := make([]IRArg, 0, len(initPostVars.Fields))
	for _, initPostVar := range initPostVars.Fields {
		initArgs = append(initArgs, IRArg{
			Name: initPostVar.Name,
			Type: initPostVar.Type,
		})
	}
	init := IRPredicate{
		Args: initArgs,
		Text: d.StartEdge.Post,
	}
	initID := init.Hash(h)

	ir.Predicates[initID] = init

	ir.Init = IRInit{
		Dst:  d.StartEdge.Dst,
		Post: initID,
		Line: d.StartEdge.Line,
	}

	for _, e := range d.Edges {
		var evParams []csdf.StateVar // TODO
		preVars := ir.States[e.Src]
		postVars := ir.States[e.Dst]

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
		guard := IRPredicate{
			Args: guardArgs,
			Text: e.Guard,
		}
		guardID := guard.Hash(h)
		ir.Predicates[guardID] = guard

		postArgs := make([]IRArg, 0, len(evParams)+len(preVars.Fields)+len(postVars.Fields))
		for _, evParam := range evParams {
			postArgs = append(postArgs, IRArg{
				Name: string(evParam.Name),
				Type: evParam.Type,
			})
		}
		for _, preVar := range preVars.Fields {
			postArgs = append(postArgs, IRArg{
				Name: preVar.Name,
				Type: preVar.Type,
			})
		}
		for _, postVar := range postVars.Fields {
			postArgs = append(postArgs, IRArg{
				Name:   postVar.Name,
				Type:   postVar.Type,
				Primed: true,
			})
		}
		post := IRPredicate{
			Args: postArgs,
			Text: e.Post,
		}
		postID := post.Hash(h)
		ir.Predicates[postID] = post

		ir.Edges = append(ir.Edges, IREdge{
			Src:         e.Src,
			Dst:         e.Dst,
			Event:       e.Event,
			EventParams: []IRArg{},
			Guard:       guardID,
			Post:        postID,
			Line:        e.Line,
		})
	}
	return ir
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

func OnlyPredicateWithUsedID(m map[IRPredicateID]IRPredicate, used map[IRPredicateID]struct{}) []IRPredicateWithID {
	ps := Predicates(m)
	res := make([]IRPredicateWithID, 0, len(ps))
	for _, p := range ps {
		if _, ok := used[p.ID]; ok {
			res = append(res, p)
		}
	}
	return res
}

// HasVars reports whether any state has a variable, in which case the json datatype is
// emitted (otherwise it would be unused).
func HasVars(ir IRLivelockFree) bool {
	for _, st := range ir.States {
		if len(st.Fields) > 0 {
			return true
		}
	}
	return false
}
