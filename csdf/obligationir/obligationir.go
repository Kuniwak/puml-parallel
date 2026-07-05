package obligationir

import (
	"hash"
	"hash/crc32"
	"slices"
	"sort"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// ObligationIR is a prover-agnostic intermediate representation of the proof
// obligation that the diagram is livelock free, i.e. no reachable state admits an
// infinite run of internal (τ) transitions. Natural-language Guard/Post predicates
// are left opaque as line-named symbols (Guard_L<line>, Post_L<line>, Init); a
// downstream generator expands this IR into Lean or Isabelle, and the predicate
// bodies are supplied separately. Both are out of scope here.
type IRLivelockFree struct {
	// StructurallyLivelockFree is true when no reachable τ-only cycle exists, in
	// which case the obligation holds regardless of the predicates.
	Structurally bool                     `json:"structurally"`
	States       map[csdf.StateID]IRState `json:"states"`    // the state space as an ADT
	Constants    []IRConst                `json:"constants"` // global opaque constants in scope
	Edges        []IREdge                 `json:"edges"`     // the labelled transitions
	Init         IRInit                   `json:"init"`
}

func (ir IRLivelockFree) CollectPredicates() []IRPredicate {
	ps := make([]IRPredicate, 0, 1+len(ir.Edges))
	ps = append(ps, ir.Init.Post)
	for _, e := range ir.Edges {
		ps = append(ps, e.Guard)
		ps = append(ps, e.Post)
	}
	return ps
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

func (a IRArg) Hash(h hash.Hash) {
	h.Write([]byte(a.Name))
	h.Write([]byte{0x00})
	h.Write([]byte(a.Type))
	h.Write([]byte{0x00})
	if a.Primed {
		h.Write([]byte{0x00})
	} else {
		h.Write([]byte{0x01})
	}
}

type IRPredicateKind string

const (
	IRPredicateKindInit  IRPredicateKind = "init"
	IRPredicateKindGuard IRPredicateKind = "guard"
	IRPredicateKindPost  IRPredicateKind = "post"
)

func ComparePredicateKind(a, b IRPredicateKind) int {
	if a == b {
		return 0
	}

	if a == IRPredicateKindInit {
		return -1
	}

	if b == IRPredicateKindInit {
		return 1
	}

	if a == IRPredicateKindGuard {
		return -1
	}

	return 1
}

// IRPredicate is an opaque predicate symbol with its argument signature and the
// verbatim natural-language text it stands for. Kind is "guard", "post", or "init".
type IRPredicate struct {
	Kind IRPredicateKind `json:"kind"`
	Args []IRArg         `json:"args"`
	Text csdf.Predicate  `json:"text"`
	Line int             `json:"int"`
}

func (p IRPredicate) Hash(h hash.Hash) {
	h.Write([]byte(p.Kind))
	for _, arg := range p.Args {
		h.Write([]byte{0x00})
		arg.Hash(h)
	}
	h.Write([]byte{0x00})
	h.Write([]byte(p.Text))
}

func (p IRPredicate) WithHash(h hash.Hash32) IRPredicateWithHash {
	p.Hash(h)
	x := h.Sum32()
	h.Reset()
	return IRPredicateWithHash{
		Predicate: p,
		Hash:      x,
	}
}

func IRPredicatesWithHash(ps []IRPredicate) []IRPredicateWithHash {
	h := crc32.NewIEEE()
	res := make([]IRPredicateWithHash, len(ps))
	for i, p := range ps {
		res[i] = p.WithHash(h)
	}
	return res
}

type IRPredicateWithHash struct {
	Predicate IRPredicate `json:"predicate"`
	Hash      uint32      `json:"hash"`
}

func ComparePredicate(a, b IRPredicate) int {
	x := a.Line - b.Line
	if x != 0 {
		return x
	}
	return ComparePredicateKind(a.Kind, b.Kind)
}

// IREdge is one transition. Guard/Post hold either a predicate symbol or the
// literal "True" when the predicate is omitted.
type IREdge struct {
	Src         csdf.StateID `json:"src"`
	Dst         csdf.StateID `json:"dst"`
	Event       csdf.Event   `json:"event"`
	EventParams []IRArg      `json:"event_params"`
	Guard       IRPredicate  `json:"guard"`
	Post        IRPredicate  `json:"post"`
	Line        int          `json:"line"` // 1-based
}

// IRInit names the start state.
type IRInit struct {
	Dst  csdf.StateID `json:"state"`
	Post IRPredicate  `json:"post"`
}

// BuildObligationIR builds the livelock-freedom proof obligation IR for d. The
// structural τ-cycle check (CheckLivelockFree) is used only to set
// StructurallyLivelockFree; the obligation itself is the global property and does
// not depend on a particular witness.
func BuildLivelockFree(d *csdf.Diagram) IRLivelockFree {
	_, free := csdf.CheckLivelockFree(d)

	ir := IRLivelockFree{
		Structurally: free,
		States:       make(map[csdf.StateID]IRState, len(d.States)),
		Constants:    []IRConst{},
		Edges:        make([]IREdge, 0, len(d.Edges)),
	}

	for _, id := range sortedStateMapIDs(d.States) {
		st := d.States[id]
		fields := make([]IRField, 0, len(st.Vars))
		for _, v := range st.Vars {
			fields = append(fields, IRField{Name: string(v.Name), Type: v.Type})
		}
		ir.States[id] = IRState{
			Fields: fields,
			Line:   st.Line,
		}
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

		ir.Edges = append(ir.Edges, IREdge{
			Src:         e.Src,
			Dst:         e.Dst,
			Event:       e.Event,
			EventParams: []IRArg{},
			Guard: IRPredicate{
				Kind: IRPredicateKindGuard,
				Args: guardArgs,
				Text: e.Guard,
				Line: e.Line,
			},
			Post: IRPredicate{
				Kind: IRPredicateKindPost,
				Args: postArgs,
				Text: e.Post,
				Line: e.Line,
			},
		})
	}

	ir.Init = IRInit{
		Dst: d.StartEdge.Dst,
		Post: IRPredicate{
			Kind: IRPredicateKindInit,
			Args: []IRArg{},
			Text: d.StartEdge.Post,
			Line: d.StartEdge.Line,
		},
	}
	return ir
}

// varsAsArgs renders a state's variables as predicate arguments, marking them
// primed when they refer to the post-state.
func varsAsArgs(d *csdf.Diagram, id csdf.StateID, primed bool) []IRArg {
	st, ok := d.States[id]
	if !ok {
		return []IRArg{}
	}
	args := make([]IRArg, 0, len(st.Vars))
	for _, v := range st.Vars {
		args = append(args, IRArg{Name: string(v.Name), Type: v.Type, Primed: primed})
	}
	return args
}

func sortedStateMapIDs(states map[csdf.StateID]csdf.State) []csdf.StateID {
	ids := make([]csdf.StateID, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
