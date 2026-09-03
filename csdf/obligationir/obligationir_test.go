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
				GuardArgs:   []IRArg{{Name: "n", Type: "Nat"}},
				Post:        541149191,
				PostArgs:    []IRArg{{Name: "n", Type: "Nat"}, {Name: "n", Type: "Nat", Primed: true}},
				Line:        5,
			},
		},
		Init: IRInit{
			Dst:      "a",
			Post:     1836624455,
			PostArgs: []IRArg{{Name: "n", Type: "Nat"}},
			Line:     4,
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
				GuardArgs:   []IRArg{},
				Post:        4261170317,
				PostArgs:    []IRArg{},
				Line:        5,
			},
		},
		Init: IRInit{
			Dst:      "s0",
			Post:     4261170317,
			PostArgs: []IRArg{},
			Line:     4,
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
			Dst:      "s0",
			Post:     3809375577,
			PostArgs: []IRArg{{Name: "ready", Type: "bool"}},
			Line:     4,
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

// TestPredicateSetSeparatesCRC32Collisions pins that two predicates whose texts
// happen to share a CRC-32 stay two predicates. The id only names a placeholder;
// letting a hash collision merge them would silently replace one of the two
// diagrams' predicates by the other's, adding or removing transitions.
func TestPredicateSetSeparatesCRC32Collisions(t *testing.T) {
	// Both texts hash to CRC-32 0x5d5a202a.
	a := IRPredicate{Args: []IRArg{}, Text: "predicate Sfo2wH6TbLqM"}
	b := IRPredicate{Args: []IRArg{}, Text: "predicate o4szAcwElsDU"}

	ps := NewPredicateSet(2)
	idA := ps.Add(a)
	idB := ps.Add(b)

	if idA == idB {
		t.Fatalf("Add(%q) and Add(%q) both returned id %v; want distinct ids", a.Text, b.Text, idA)
	}
	if got := ps.Map()[idA]; got.Text != a.Text {
		t.Errorf("Map()[%v].Text = %q, want %q", idA, got.Text, a.Text)
	}
	if got := ps.Map()[idB]; got.Text != b.Text {
		t.Errorf("Map()[%v].Text = %q, want %q", idB, got.Text, b.Text)
	}
}

// TestPredicateSetSharesEqualPredicates pins the other half: an identical text
// and signature must still collapse to one placeholder, which is what lets a
// predicate occurring in both diagrams be formalised once.
func TestPredicateSetSharesEqualPredicates(t *testing.T) {
	p := IRPredicate{Args: []IRArg{{Name: "x", Type: "nat"}}, Text: "x > 0"}
	q := IRPredicate{Args: []IRArg{{Name: "y", Type: "nat"}}, Text: "x > 0"}

	ps := NewPredicateSet(2)
	if idP, idQ := ps.Add(p), ps.Add(q); idP != idQ {
		t.Errorf("Add() returned %v and %v for the same text and signature; want one id", idP, idQ)
	}
	if len(ps.Map()) != 1 {
		t.Errorf("len(Map()) = %d, want 1", len(ps.Map()))
	}
}
