package obligationir

import (
	"hash/crc32"
	"reflect"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
)

func TestBuildLivelockFreeTauSelfLoopWithVars(t *testing.T) {
	// Setup: a guarded tau self-loop carrying a state variable. The cycle is a
	// structural candidate, so the obligation is non-trivial and the written
	// predicates become opaque line-named symbols.
	d := csdf.MustParse(`@startuml
state "a" as a
a: n ; Nat
[*] --> a
a --> a : tau ; n > 0 ; n' = n - 1
@enduml
`)

	want := IRLivelockFree{
		Structurally: false,
		States: map[csdf.StateID]IRState{
			"a": {
				Fields: []IRField{{Name: "n", Type: "Nat"}},
				Line:   2,
			},
		},
		Predicates: map[IRPredicateID]IRPredicate{
			541149191: {
				Args: []IRArg{
					{Name: "n", Type: "Nat", Primed: false},
					{Name: "n", Type: "Nat", Primed: true},
				},
				Text: "n' = n - 1",
			},
			2223308920: {
				Args: []IRArg{
					{Name: "n", Type: "Nat", Primed: false},
				},
				Text: "n > 0",
			},
			1836624455: {
				Args: []IRArg{
					{Name: "n", Type: "Nat", Primed: false},
				},
				Text: csdf.PredicateTrue,
			},
		},
		Constants: []IRConst{},
		Edges: []IREdge{
			{
				Src:         "a",
				Dst:         "a",
				Event:       "tau",
				EventParams: []IRArg{},
				Guard:       2223308920,
				Post:        541149191,
				Line:        5,
			},
		},
		Init: IRInit{
			Dst:  "a",
			Post: 1836624455,
			Line: 4,
		},
	}

	// Execute
	got := BuildLivelockFree(d)

	// Assert
	if !reflect.DeepEqual(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestBuildLivelockFreeStructurallyFreeDefaults(t *testing.T) {
	// Setup: a visible-only chain has no tau cycle (structurally livelock free),
	// and its omitted guard/post default to the literal True (no opaque symbol).
	d := csdf.MustParse(`@startuml
state "s0" as s0
state "s1" as s1
[*] --> s0
s0 --> s1 : a
@enduml
`)

	want := IRLivelockFree{
		Structurally: true,
		States: map[csdf.StateID]IRState{
			"s0": {Fields: []IRField{}, Line: 2},
			"s1": {Fields: []IRField{}, Line: 3},
		},
		Predicates: map[IRPredicateID]IRPredicate{
			4261170317: {
				Args: []IRArg{},
				Text: csdf.PredicateTrue,
			},
		},
		Constants: []IRConst{},
		Edges: []IREdge{
			{
				Src:         "s0",
				Dst:         "s1",
				Event:       "a",
				EventParams: []IRArg{},
				Guard:       4261170317,
				Post:        4261170317,
				Line:        5,
			},
		},
		Init: IRInit{
			Dst:  "s0",
			Post: 4261170317,
			Line: 4,
		},
	}

	// Execute
	got := BuildLivelockFree(d)

	// Assert
	if !reflect.DeepEqual(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestBuildLivelockFreeNamesInitPredicate(t *testing.T) {
	// Setup: a non-default start post becomes the opaque Init predicate over the
	// start state's variables.
	d := csdf.MustParse(`@startuml
state "s0" as s0
s0: ready ; bool
[*] --> s0 : initialized
@enduml
`)

	// Execute
	got := BuildLivelockFree(d)

	// Assert
	want := IRLivelockFree{
		Structurally: true,
		States: map[csdf.StateID]IRState{
			"s0": {
				Fields: []IRField{
					{Name: "ready", Type: "bool"},
				},
				Line: 2,
			},
		},
		Predicates: map[IRPredicateID]IRPredicate{
			3809375577: {
				Args: []IRArg{
					{Name: "ready", Type: "bool"},
				},
				Text: "initialized",
			},
		},
		Constants: []IRConst{},
		Edges:     []IREdge{},
		Init: IRInit{
			Dst:  "s0",
			Post: 3809375577,
			Line: 4,
		},
	}

	if !reflect.DeepEqual(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestIRPredicateHash(t *testing.T) {
	p := IRPredicate{
		Args: []IRArg{},
		Text: "true",
	}

	h := crc32.NewIEEE()
	t1 := p.Hash(h)
	t2 := p.Hash(h)

	if t1 != t2 {
		t.Errorf("want %d, got %d", t1, t2)
	}
}
