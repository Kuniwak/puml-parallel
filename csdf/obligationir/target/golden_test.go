package target_test

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligationir/target"
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

// compile writes the obligation of the diagrams loaded for a case.
type compile func(w io.Writer, ds []*core.Diagram, n target.Name) error

func livelock(w io.Writer, ds []*core.Diagram, n target.Name) error {
	return target.CompileDiagram(w, ds[0], n)
}

func refinement(mode obligationir.IRRefinementMode) compile {
	return func(w io.Writer, ds []*core.Diagram, n target.Name) error {
		return target.CompileDiagrams(w, mode, ds[0], ds[1], n)
	}
}

// TestTheObligationsAreTheExpectedOnes holds each back end to the obligation
// expected of each diagram, or pair of diagrams, of testdata/formula. The
// provers check the same diagrams (provercheck), so a golden file the
// generator wrote is still one a prover accepted.
func TestTheObligationsAreTheExpectedOnes(t *testing.T) {
	testCases := map[string]struct {
		Golden   string
		Diagrams []string
		Compile  compile
	}{
		"livelock: a tau countdown spells its clauses":                         {"countdown", []string{"countdown"}, livelock},
		"livelock: maps, a received field and an ∃ under the axioms they need": {"map", []string{"map"}, livelock},
		"livelock: every builtin, defined, and its meaning proved":             {"builtins", []string{"builtins"}, livelock},
		"livelock: literals, each spelled as the value it is":                  {"literals", []string{"literals"}, livelock},
		"livelock: a diagram with no tau cycle has no obligation":              {"visible", []string{"visible"}, livelock},
		"refinement: values on events, in traces":                              {"refinement/counter_countdown.t", []string{"counter", "countdown"}, refinement(obligationir.IRRefinementModeTrace)},
		"refinement: a receive and a send, in stable failures":                 {"refinement/counter_counter.f", []string{"counter", "counter"}, refinement(obligationir.IRRefinementModeStableFailure)},
		"refinement: axioms and assumptions, divergence-freedom":               {"refinement/map_countdown.fd", []string{"map", "countdown"}, refinement(obligationir.IRRefinementModeFailuresDivergence)},
	}

	for name, tc := range testCases {
		for backEnd, ext := range backEnds {
			t.Run(name+"/"+string(backEnd), func(t *testing.T) {
				// Setup
				golden := filepath.Join(dir, tc.Golden+ext)
				ds := make([]*core.Diagram, len(tc.Diagrams))
				for i, name := range tc.Diagrams {
					ds[i] = load(t, name)
				}
				var sb strings.Builder

				// Execute
				err := tc.Compile(&sb, ds, backEnd)
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
