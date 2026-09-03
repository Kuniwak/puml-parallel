package obligationir

import (
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// IRRefinementMode selects the CSP model the refinement is stated in.
type IRRefinementMode string

const (
	IRRefinementModeTrace              IRRefinementMode = "trace"
	IRRefinementModeStableFailure      IRRefinementMode = "stable-failure"
	IRRefinementModeFailuresDivergence IRRefinementMode = "failures-divergence"
)

// IRSide is one diagram of a refinement obligation. Its predicates live in the
// shared IRRefinement.Predicates map, so a predicate occurring in both diagrams
// is a single placeholder that is filled once.
type IRSide struct {
	States map[csdf.StateID]IRState `json:"states"`
	Edges  []IREdge                 `json:"edges"`
	Init   IRInit                   `json:"init"`
	// End is nil when the diagram cannot terminate.
	End *IREnd `json:"end,omitempty"`
	// StructurallyLivelockFree is nil outside failures-divergence mode, where
	// divergence is not observable at all; there it is true when the side has no
	// reachable tau-cycle and so discharges its divergence-freedom obligation
	// structurally.
	StructurallyLivelockFree *bool `json:"structurally_livelock_free,omitempty"`
}

// IREnd is the guarded successful termination of a diagram (its EndEdge).
// GuardArgs is this occurrence's own argument list; see IREdge.
type IREnd struct {
	Src       csdf.StateID  `json:"src"`
	Guard     IRPredicateID `json:"guard"`
	GuardArgs []IRArg       `json:"guard_args"`
	Line      int           `json:"line"` // 1-based
}

// IRRefinement is a prover-agnostic intermediate representation of the proof
// obligation that Spec refines Impl in the selected model, i.e. every behaviour
// of the Impl diagram is allowed by the Spec diagram (FDR's direction).
// Natural-language Guard/Post predicates are left opaque exactly as in
// IRLivelockFree, and under the same pred_<id> spelling, so a diagram formalised
// for the livelock obligation carries over verbatim.
type IRRefinement struct {
	Mode       IRRefinementMode              `json:"mode"`
	Alphabet   []csdf.Event                  `json:"alphabet"` // union of both sides' visible events, sorted
	Predicates map[IRPredicateID]IRPredicate `json:"predicates"`
	Constants  []IRConst                     `json:"constants"`
	Spec       IRSide                        `json:"spec"`
	Impl       IRSide                        `json:"impl"`
}

// BuildRefinement builds the refinement proof obligation IR for spec ⊑ impl in
// the given model.
func BuildRefinement(mode IRRefinementMode, spec, impl *csdf.Diagram) IRRefinement {
	ps := NewPredicateSet((len(spec.Edges)+len(impl.Edges))*2 + 2)

	return IRRefinement{
		Mode:       mode,
		Alphabet:   alphabet(spec, impl),
		Constants:  []IRConst{},
		Spec:       buildSide(ps, spec, mode),
		Impl:       buildSide(ps, impl, mode),
		Predicates: ps.Map(),
	}
}

func buildSide(ps *PredicateSet, d *csdf.Diagram, mode IRRefinementMode) IRSide {
	states := BuildStates(d)
	side := IRSide{
		States: states,
		Init:   BuildInit(ps, d, states),
		Edges:  BuildEdges(ps, d, states),
		End:    BuildEnd(ps, d, states),
	}
	if mode == IRRefinementModeFailuresDivergence {
		_, free := csdf.CheckLivelockFree(d)
		side.StructurallyLivelockFree = &free
	}
	return side
}

// alphabet is the sorted union of both diagrams' visible events. τ is not part of
// it: it is internal, and the encoding hides it.
func alphabet(ds ...*csdf.Diagram) []csdf.Event {
	seen := make(map[csdf.Event]struct{})
	for _, d := range ds {
		for _, e := range d.Edges {
			if e.Event == csdf.Tau {
				continue
			}
			seen[e.Event] = struct{}{}
		}
	}

	res := make([]csdf.Event, 0, len(seen))
	for ev := range seen {
		res = append(res, ev)
	}
	slices.Sort(res)
	return res
}
