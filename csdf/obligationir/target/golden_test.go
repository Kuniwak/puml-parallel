package target_test

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite the expected obligations")

// dir holds the diagrams the golden obligations are of.
var dir = filepath.Join("..", "testdata", "formula")

// backEnds are the targets each obligation is expected in, and the extension
// of the file it is expected in.
var backEnds = map[target.Name]string{
	target.NameIsabelle: ".thy",
	target.NameLean:     ".lean",
	target.NameIRJSON:   ".json",
}

func load(t *testing.T, name string) *core.Diagram {
	t.Helper()
	d, err := coreio.Load(filepath.Join(dir, name+".puml"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func livelock(name string) func(*testing.T, io.Writer, target.Name) error {
	return func(t *testing.T, w io.Writer, n target.Name) error {
		return target.CompileDiagram(w, load(t, name), n)
	}
}

func refinement(mode obligationir.IRRefinementMode, spec, impl string) func(*testing.T, io.Writer, target.Name) error {
	return func(t *testing.T, w io.Writer, n target.Name) error {
		return target.CompileDiagrams(w, mode, load(t, spec), load(t, impl), n)
	}
}

// TestTheObligationsAreTheExpectedOnes holds each back end to the obligation
// expected of each diagram, or pair of diagrams, of testdata/formula. The
// provers check the same diagrams (provercheck), so a golden file the
// generator wrote is still one a prover accepted.
func TestTheObligationsAreTheExpectedOnes(t *testing.T) {
	testCases := map[string]struct {
		Golden  string
		Compile func(*testing.T, io.Writer, target.Name) error
	}{
		"livelock: a tau countdown spells its clauses":                         {"countdown", livelock("countdown")},
		"livelock: maps, a received field and an ∃ under the axioms they need": {"map", livelock("map")},
		"livelock: a diagram with no tau cycle has no obligation":              {"visible", livelock("visible")},
		"refinement: values on events, in traces":                              {"refinement/counter_countdown.t", refinement(obligationir.IRRefinementModeTrace, "counter", "countdown")},
		"refinement: a receive and a send, in stable failures":                 {"refinement/counter_counter.f", refinement(obligationir.IRRefinementModeStableFailure, "counter", "counter")},
		"refinement: axioms and assumptions, divergence-freedom":               {"refinement/map_countdown.fd", refinement(obligationir.IRRefinementModeFailuresDivergence, "map", "countdown")},
	}

	for name, tc := range testCases {
		for backEnd, ext := range backEnds {
			t.Run(name+"/"+string(backEnd), func(t *testing.T) {
				// Setup
				golden := filepath.Join(dir, tc.Golden+ext)
				var sb strings.Builder

				// Execute
				err := tc.Compile(t, &sb, backEnd)
				if err != nil {
					t.Fatal(err)
				}
				if *update {
					if err := os.WriteFile(golden, []byte(sb.String()), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}

				// Assert
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(string(want), sb.String()); diff != "" {
					t.Error(diff)
				}
			})
		}
	}
}
