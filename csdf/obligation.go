package csdf

import (
	"fmt"
	"sort"
)

// ObligationIR is a prover-agnostic intermediate representation of the proof
// obligation that the diagram is livelock free, i.e. no reachable state admits an
// infinite run of internal (τ) transitions. Natural-language Guard/Post predicates
// are left opaque as line-named symbols (Guard_L<line>, Post_L<line>, Init); a
// downstream generator expands this IR into Lean or Isabelle, and the predicate
// bodies are supplied separately. Both are out of scope here.
type ObligationIR struct {
	Goal string `json:"goal"` // always "livelock_free"
	// StructurallyLivelockFree is true when no reachable τ-only cycle exists, in
	// which case the obligation holds regardless of the predicates.
	StructurallyLivelockFree bool          `json:"structurally_livelock_free"`
	States                   []IRState     `json:"states"`     // the state space as an ADT
	Constants                []IRConst     `json:"constants"`  // global opaque constants in scope
	Predicates               []IRPredicate `json:"predicates"` // opaque predicate symbols + signatures
	Edges                    []IREdge      `json:"edges"`      // the labelled transitions
	Init                     IRInit        `json:"init"`
}

// IRState is one ADT constructor: a diagram state whose fields are its variables.
type IRState struct {
	Ctor   string    `json:"ctor"`
	Fields []IRField `json:"fields"`
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
	Sym  string  `json:"sym"`
	Kind string  `json:"kind"`
	Line int     `json:"line"`
	Args []IRArg `json:"args"`
	Text string  `json:"text"`
}

// IREdge is one transition. Guard/Post hold either a predicate symbol or the
// literal "True" when the predicate is omitted.
type IREdge struct {
	Line        int     `json:"line"`
	Src         string  `json:"src"`
	Dst         string  `json:"dst"`
	Event       string  `json:"event"`
	Tau         bool    `json:"tau"`
	EventParams []IRArg `json:"event_params"`
	Guard       string  `json:"guard"`
	Post        string  `json:"post"`
}

// IRInit names the start state and its initialisation predicate ("Init" or "True").
type IRInit struct {
	State string `json:"state"`
	Pred  string `json:"pred"`
}

// BuildObligationIR builds the livelock-freedom proof obligation IR for d. The
// structural τ-cycle check (CheckLivelockFree) is used only to set
// StructurallyLivelockFree; the obligation itself is the global property and does
// not depend on a particular witness.
func BuildObligationIR(d *Diagram) ObligationIR {
	_, free := CheckLivelockFree(d)

	ir := ObligationIR{
		Goal:                     "livelock_free",
		StructurallyLivelockFree: free,
		States:                   make([]IRState, 0, len(d.States)),
		Constants:                []IRConst{},
		Predicates:               []IRPredicate{},
		Edges:                    make([]IREdge, 0, len(d.Edges)),
	}

	for _, id := range sortedStateMapIDs(d.States) {
		st := d.States[id]
		fields := make([]IRField, 0, len(st.Vars))
		for _, v := range st.Vars {
			fields = append(fields, IRField{Name: string(v.Name), Type: v.Type})
		}
		ir.States = append(ir.States, IRState{Ctor: string(id), Fields: fields})
	}

	for _, e := range d.Edges {
		guardSym := "True"
		if !isDefaultPred(e.Guard) {
			guardSym = fmt.Sprintf("Guard_L%d", e.Line)
			ir.Predicates = append(ir.Predicates, IRPredicate{
				Sym:  guardSym,
				Kind: "guard",
				Line: e.Line,
				Args: varsAsArgs(d, e.Src, false),
				Text: e.Guard,
			})
		}
		postSym := "True"
		if !isDefaultPred(e.Post) {
			postSym = fmt.Sprintf("Post_L%d", e.Line)
			args := varsAsArgs(d, e.Src, false)
			args = append(args, varsAsArgs(d, e.Dst, true)...)
			ir.Predicates = append(ir.Predicates, IRPredicate{
				Sym:  postSym,
				Kind: "post",
				Line: e.Line,
				Args: args,
				Text: e.Post,
			})
		}
		ir.Edges = append(ir.Edges, IREdge{
			Line:        e.Line,
			Src:         string(e.Src),
			Dst:         string(e.Dst),
			Event:       string(e.Event),
			Tau:         e.Event == Tau,
			EventParams: []IRArg{},
			Guard:       guardSym,
			Post:        postSym,
		})
	}

	initPred := "True"
	if !isDefaultPred(d.StartEdge.Post) {
		initPred = "Init"
		ir.Predicates = append(ir.Predicates, IRPredicate{
			Sym:  "Init",
			Kind: "init",
			Line: d.StartEdge.Line,
			Args: varsAsArgs(d, d.StartEdge.Dst, false),
			Text: d.StartEdge.Post,
		})
	}
	ir.Init = IRInit{State: string(d.StartEdge.Dst), Pred: initPred}

	return ir
}

// isDefaultPred reports whether a predicate string is the omitted/default value,
// which renders as the literal True rather than an opaque symbol. The capitalised
// "True"/"False" written by an author are ordinary natural-language predicates.
func isDefaultPred(s string) bool {
	return s == "" || s == True
}

// varsAsArgs renders a state's variables as predicate arguments, marking them
// primed when they refer to the post-state.
func varsAsArgs(d *Diagram, id StateID, primed bool) []IRArg {
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

func sortedStateMapIDs(states map[StateID]State) []StateID {
	ids := make([]StateID, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
