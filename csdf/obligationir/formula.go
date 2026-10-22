package obligationir

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/Kuniwak/puml-parallel/csdf/obligation"
)

// LivelockFree is the livelock-freedom obligation of a diagram whose clauses
// are csdf/clause formulas (docs/CLAUSE_DESIGN.md, O8): no reachable state
// admits an infinite run of tau steps. Each clause is the formula of
// csdf/logic that csdf/obligation states, so a back end spells it rather than
// leaving it opaque: an atom is an uninterpreted predicate applied to its
// holes, a builtin an equality or a csdf_* predicate whose meaning Axioms
// states, and an ∃ a quantifier of its type.
type LivelockFree struct {
	// Structurally is true when no tau cycle is reachable in the graph of
	// the states, so the obligation holds whatever the clauses say.
	Structurally bool `json:"structurally"`
	// States are the states in the order the diagram declares them, each
	// with its state variables.
	States []State `json:"states"`
	// Init is the start state; InitPost the post of the start edge, over the
	// values after it (x').
	Init     string     `json:"init"`
	InitPost logic.Tree `json:"init_post"`
	Edges    []Edge     `json:"edges"`
	// Axioms are what the csdf_* predicates of the clauses mean
	// (obligation.Axioms); a back end states the theorem under them.
	Axioms []logic.Tree `json:"axioms"`
	// Assumptions are the diagram's assume notes, hypotheses of the theorem
	// like the axioms. Never null, only empty.
	Assumptions []IRAssumption `json:"assumptions"`
}

// State is a state and the state variables it holds.
type State struct {
	Name string   `json:"name"`
	Vars []string `json:"vars"`
}

// Edge is a step of the diagram: from Src to Dst, internal when Tau.
// Received are the fields its event receives, which Step reads and the step
// chooses; Step is its guard and post, over the values before it (x), the
// received fields and the values after it (x').
type Edge struct {
	Src      string     `json:"src"`
	Dst      string     `json:"dst"`
	Tau      bool       `json:"tau"`
	Received []string   `json:"received"`
	Step     logic.Tree `json:"step"`
}

// BuildFormulaLivelockFree states the livelock-freedom obligation of d.
func BuildFormulaLivelockFree(d *core.Diagram) (LivelockFree, error) {
	post, err := initPost(d)
	if err != nil {
		return LivelockFree{}, fmt.Errorf("obligationir.BuildFormulaLivelockFree: %w", err)
	}
	clauses := []clause.Formula{d.InitPost}
	edges := make([]Edge, 0, len(d.Edges))
	for _, e := range d.Edges {
		step, err := obligation.Step(*d, e)
		if err != nil {
			return LivelockFree{}, fmt.Errorf("obligationir.BuildFormulaLivelockFree: %w", err)
		}
		var received []string
		for _, f := range e.Event.Fields {
			if !f.Send {
				received = append(received, f.Name)
			}
		}
		edges = append(edges, Edge{Src: e.Src, Dst: e.Dst, Tau: e.Event.Channel == core.Tau, Received: received, Step: logic.TreeOf(step)})
		clauses = append(clauses, e.Guard, e.Post)
	}
	states := make([]State, len(d.States))
	for i, s := range d.States {
		states[i] = State{Name: s, Vars: slices.Clone(d.Vars[s])}
	}
	assumptions := make([]IRAssumption, 0, len(d.Assumes))
	for _, a := range d.Assumes {
		assumptions = append(assumptions, IRAssumption{ID: a.ID, Text: csdf.Predicate(a.Text)})
	}
	return LivelockFree{
		Structurally: !tauCycleReachable(d),
		States:       states,
		Init:         d.Init,
		InitPost:     logic.TreeOf(post),
		Edges:        edges,
		Axioms:       trees(obligation.Axioms(clauses...)),
		Assumptions:  assumptions,
	}, nil
}

// initPost is the post of the start edge, which reads only the values after
// it of the variables the start state holds.
func initPost(d *core.Diagram) (logic.Formula, error) {
	for _, v := range clause.FreeVars(d.InitPost) {
		if !v.Primed || !slices.Contains(d.Vars[d.Init], v.Name) {
			return nil, fmt.Errorf("the start post reads %s, which is no value after the start that %s holds", obligation.Var(v), d.Init)
		}
	}
	f, err := obligation.Formula(d.InitPost)
	if err != nil {
		return nil, fmt.Errorf("the start post: %w", err)
	}
	return logic.Simplify(f), nil
}

// tauCycleReachable reports whether the graph of the states, every edge
// taken whatever its clauses, reaches a cycle of tau edges from the start.
func tauCycleReachable(d *core.Diagram) bool {
	reached := map[string]bool{d.Init: true}
	todo := []string{d.Init}
	for len(todo) > 0 {
		s := todo[0]
		todo = todo[1:]
		for _, e := range d.Edges {
			if e.Src == s && !reached[e.Dst] {
				reached[e.Dst] = true
				todo = append(todo, e.Dst)
			}
		}
	}
	// A state is on a tau cycle when it reaches itself by tau edges alone.
	for s := range reached {
		seen := map[string]bool{}
		todo := []string{s}
		for len(todo) > 0 {
			t := todo[0]
			todo = todo[1:]
			for _, e := range d.Edges {
				if e.Src != t || e.Event.Channel != core.Tau {
					continue
				}
				if e.Dst == s {
					return true
				}
				if !seen[e.Dst] {
					seen[e.Dst] = true
					todo = append(todo, e.Dst)
				}
			}
		}
	}
	return false
}

func trees(fs []logic.Formula) []logic.Tree {
	out := make([]logic.Tree, len(fs))
	for i, f := range fs {
		out[i] = logic.TreeOf(f)
	}
	return out
}
