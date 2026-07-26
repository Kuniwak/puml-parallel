package obligationir

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
)

func TestBuildRefinementTraceSharesOnePredicateLayer(t *testing.T) {
	// Setup: two ground diagrams. Every guard/post is the omitted default, so all
	// of them hash to the one no-argument True predicate: the shared placeholder
	// layer is filled once and serves both sides.
	spec := csdf.MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)
	impl := csdf.MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
s1 --> s0 : b
@enduml
`)

	truePred := IRPredicateID(4261170317)
	want := IRRefinement{
		Mode: IRRefinementModeTrace,
		// The union of both sides' visible events, sorted: refusal information
		// only makes sense against one shared alphabet.
		Alphabet: []csdf.Event{"a", "b"},
		Predicates: map[IRPredicateID]IRPredicate{
			truePred: {Args: []IRArg{}, Text: csdf.PredicateTrue},
		},
		Constants: []IRConst{},
		Spec: IRSide{
			States: map[csdf.StateID]IRState{
				"s0": {Fields: []IRField{}, Line: 2},
				"s1": {Fields: []IRField{}, Line: 3},
			},
			Edges: []IREdge{
				{Src: "s0", Dst: "s1", Event: "a", EventParams: []IRArg{}, Guard: truePred, Post: truePred, Line: 5},
			},
			Init: IRInit{Dst: "s0", Post: truePred, Line: 4},
		},
		Impl: IRSide{
			States: map[csdf.StateID]IRState{
				"s0": {Fields: []IRField{}, Line: 2},
				"s1": {Fields: []IRField{}, Line: 3},
			},
			Edges: []IREdge{
				{Src: "s0", Dst: "s1", Event: "a", EventParams: []IRArg{}, Guard: truePred, Post: truePred, Line: 5},
				{Src: "s1", Dst: "s0", Event: "b", EventParams: []IRArg{}, Guard: truePred, Post: truePred, Line: 6},
			},
			Init: IRInit{Dst: "s0", Post: truePred, Line: 4},
		},
	}

	// Execute
	got := BuildRefinement(IRRefinementModeTrace, spec, impl)

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestBuildRefinementCarriesTheEndEdgeAsGuardedTermination(t *testing.T) {
	// Setup: an EndEdge is CSP successful termination, guarded by its own
	// predicate, so the IR has to keep it (and register its guard in the shared
	// predicate layer). A diagram without one terminates nowhere.
	withEnd := csdf.MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> [*] : done
@enduml
`)
	withoutEnd := csdf.MustParse(`@startuml
state "s0" as s0
[*] --> s0
s0 --> s0 : a
@enduml
`)

	// Execute
	got := BuildRefinement(IRRefinementModeTrace, withEnd, withoutEnd)

	// Assert
	if got.Spec.End == nil {
		t.Fatal("want the spec's end edge to be kept, got nil")
	}
	if got.Spec.End.Src != "s0" || got.Spec.End.Line != 4 {
		t.Errorf("want end edge s0 on line 4, got %#v", got.Spec.End)
	}
	if text := got.Predicates[got.Spec.End.Guard].Text; text != "done" {
		t.Errorf("want the end guard text %q, got %q", "done", text)
	}
	if got.Impl.End != nil {
		t.Errorf("want no end edge on the impl side, got %#v", got.Impl.End)
	}
}

func TestBuildRefinementFailuresDivergenceCarriesPerSideLivelockFreedom(t *testing.T) {
	// Setup: CSP-Prover has no FD model, so fd reduces to <=F plus a
	// divergence-freedom obligation per side. A side whose tau relation has no
	// reachable cycle discharges that obligation structurally.
	diverging := csdf.MustParse(`@startuml
state "a" as a
[*] --> a
a --> a : tau
@enduml
`)
	visible := csdf.MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	// Execute
	got := BuildRefinement(IRRefinementModeFailuresDivergence, diverging, visible)

	// Assert
	if got.Spec.StructurallyLivelockFree == nil || got.Impl.StructurallyLivelockFree == nil {
		t.Fatalf("want both sides to carry livelock freedom in fd mode, got %#v / %#v",
			got.Spec.StructurallyLivelockFree, got.Impl.StructurallyLivelockFree)
	}
	if *got.Spec.StructurallyLivelockFree {
		t.Error("want spec structurally livelock free = false (reachable tau cycle)")
	}
	if !*got.Impl.StructurallyLivelockFree {
		t.Error("want impl structurally livelock free = true (no tau edge at all)")
	}
}

func TestBuildRefinementOmitsLivelockFreedomOutsideFailuresDivergence(t *testing.T) {
	// The trace and stable-failures models never observe divergence, so the
	// livelock fields would be noise there.
	d := csdf.MustParse(`@startuml
state "a" as a
[*] --> a
a --> a : tau
@enduml
`)

	for _, mode := range []IRRefinementMode{IRRefinementModeTrace, IRRefinementModeStableFailure} {
		t.Run(string(mode), func(t *testing.T) {
			got := BuildRefinement(mode, d, d)
			if got.Spec.StructurallyLivelockFree != nil || got.Impl.StructurallyLivelockFree != nil {
				t.Errorf("want nil livelock fields for mode %s, got %#v / %#v",
					mode, got.Spec.StructurallyLivelockFree, got.Impl.StructurallyLivelockFree)
			}
		})
	}
}
