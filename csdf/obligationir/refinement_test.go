package obligationir_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
)

func mustRefinement(t *testing.T) obligationir.Refinement {
	t.Helper()
	spec, err := coreio.Load("testdata/formula/counter.puml")
	if err != nil {
		t.Fatal(err)
	}
	impl, err := coreio.Load("testdata/formula/countdown.puml")
	if err != nil {
		t.Fatal(err)
	}
	r, err := obligationir.BuildRefinement(obligationir.IRRefinementModeTrace, spec, impl)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRefinementValidateRefusesAnIROfNoPairOfDiagrams(t *testing.T) {
	testCases := map[string]func(r *obligationir.Refinement){
		"another version":                          func(r *obligationir.Refinement) { r.Version = 0 },
		"an unknown model":                         func(r *obligationir.Refinement) { r.Mode = "fdr" },
		"a side Validate refuses":                  func(r *obligationir.Refinement) { r.Impl.Init = "nowhere" },
		"an alphabet missing a channel":            func(r *obligationir.Refinement) { r.Alphabet = r.Alphabet[1:] },
		"an alphabet with a channel's arity wrong": func(r *obligationir.Refinement) { r.Alphabet[0].Arity++ },
	}

	if err := mustRefinement(t).Validate(); err != nil {
		t.Fatalf("the valid IR is refused: %v", err)
	}
	for name, broken := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			r := mustRefinement(t)
			broken(&r)

			// Execute
			err := r.Validate()

			// Assert
			if err == nil {
				t.Error("err = nil, want the IR refused")
			}
		})
	}
}

func TestBuildRefinementTakesTheChannelsOfBothSides(t *testing.T) {
	// Execute
	r := mustRefinement(t)

	// Assert: the tau of either side is no channel.
	want := []obligationir.Channel{{Name: "inc", Arity: 1}, {Name: "show", Arity: 1}}
	if len(r.Alphabet) != len(want) || r.Alphabet[0] != want[0] || r.Alphabet[1] != want[1] {
		t.Errorf("alphabet = %v, want %v", r.Alphabet, want)
	}
}
