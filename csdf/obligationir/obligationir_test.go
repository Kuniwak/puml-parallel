package obligationir

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
)

func mustParse(t *testing.T, input string) *csdf.Diagram {
	t.Helper()
	d, err := csdf.ParseDiagram([]byte(input))
	if err != nil {
		t.Fatalf("ParseDiagram() error = %v", err)
	}
	return d
}

func TestBuildObligationIRTauSelfLoopWithVars(t *testing.T) {
	// Setup: a guarded tau self-loop carrying a state variable. The cycle is a
	// structural candidate, so the obligation is non-trivial and the written
	// predicates become opaque line-named symbols.
	d := mustParse(t, `@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := ObligationIR{
		Goal:                     "livelock_free",
		StructurallyLivelockFree: false,
		States: []IRState{
			{Ctor: "a", Fields: []IRField{{Name: "n", Type: "Nat"}}},
		},
		Constants: []IRConst{},
		Predicates: []IRPredicate{
			{Sym: "Guard_L5", Kind: "guard", Line: 5,
				Args: []IRArg{{Name: "n", Type: "Nat", Primed: false}},
				Text: "n > 0"},
			{Sym: "Post_L5", Kind: "post", Line: 5,
				Args: []IRArg{
					{Name: "n", Type: "Nat", Primed: false},
					{Name: "n", Type: "Nat", Primed: true},
				},
				Text: "n' = n - 1"},
		},
		Edges: []IREdge{
			{Line: 5, Src: "a", Dst: "a", Event: "tau", Tau: true,
				EventParams: []IRArg{}, Guard: "Guard_L5", Post: "Post_L5"},
		},
		Init: IRInit{State: "a", Pred: "True"},
	}

	// Execute
	got := BuildObligationIR(d)

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestBuildObligationIRStructurallyFreeDefaults(t *testing.T) {
	// Setup: a visible-only chain has no tau cycle (structurally livelock free),
	// and its omitted guard/post default to the literal True (no opaque symbol).
	d := mustParse(t, `@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	want := ObligationIR{
		Goal:                     "livelock_free",
		StructurallyLivelockFree: true,
		States: []IRState{
			{Ctor: "s0", Fields: []IRField{}},
			{Ctor: "s1", Fields: []IRField{}},
		},
		Constants:  []IRConst{},
		Predicates: []IRPredicate{},
		Edges: []IREdge{
			{Line: 5, Src: "s0", Dst: "s1", Event: "a", Tau: false,
				EventParams: []IRArg{}, Guard: "True", Post: "True"},
		},
		Init: IRInit{State: "s0", Pred: "True"},
	}

	// Execute
	got := BuildObligationIR(d)

	// Assert
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestBuildObligationIRNamesInitPredicate(t *testing.T) {
	// Setup: a non-default start post becomes the opaque Init predicate over the
	// start state's variables.
	d := mustParse(t, `@startuml
state "s0" as s0
s0: ready ; bool
[*] --> s0 : initialized
@enduml
`)

	got := BuildObligationIR(d)

	// Assert
	if got.Init != (IRInit{State: "s0", Pred: "Init"}) {
		t.Errorf("Init = %+v, want {State:s0 Pred:Init}", got.Init)
	}
	wantPred := IRPredicate{
		Sym: "Init", Kind: "init", Line: 4,
		Args: []IRArg{{Name: "ready", Type: "bool", Primed: false}},
		Text: "initialized",
	}
	if len(got.Predicates) != 1 {
		t.Fatalf("Predicates = %#v, want one Init predicate", got.Predicates)
	}
	if diff := cmp.Diff(wantPred, got.Predicates[0]); diff != "" {
		t.Error(diff)
	}
}
