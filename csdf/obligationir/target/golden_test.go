package target_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/target"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite the expected obligations")

// TestCompileDiagramWritesTheExpectedObligation holds each back end to the
// obligation expected of each diagram in testdata/formula. The provers check
// the same files (provercheck), so a golden file the generator wrote is
// still one a prover accepted.
func TestCompileDiagramWritesTheExpectedObligation(t *testing.T) {
	diagrams := map[string]string{
		"a tau countdown spells its clauses":                         "countdown",
		"maps, a received field and an ∃ under the axioms they need": "map",
		"a diagram with no tau cycle has no obligation":              "visible",
	}
	backEnds := map[target.Name]string{
		target.NameIsabelle: ".thy",
		target.NameLean:     ".lean",
		target.NameIRJSON:   ".json",
	}

	for name, file := range diagrams {
		for backEnd, ext := range backEnds {
			t.Run(name+"/"+string(backEnd), func(t *testing.T) {
				// Setup
				base := filepath.Join("..", "testdata", "formula", file)
				d, err := coreio.Load(base + ".puml")
				if err != nil {
					t.Fatal(err)
				}
				var sb strings.Builder

				// Execute
				err = target.CompileDiagram(&sb, d, backEnd)

				// Assert
				if err != nil {
					t.Fatal(err)
				}
				if *update {
					if err := os.WriteFile(base+ext, []byte(sb.String()), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(base + ext)
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

// TestCompileDiagramsWritesTheExpectedObligation holds each back end to the
// refinement obligation expected of each pair of diagrams of testdata/formula
// in a model. The provers check the same pairs (provercheck).
func TestCompileDiagramsWritesTheExpectedObligation(t *testing.T) {
	pairs := map[string]struct {
		Spec, Impl, Golden string
		Mode               obligationir.IRRefinementMode
	}{
		"values on events, in traces":                {"counter", "countdown", "counter_countdown.t", obligationir.IRRefinementModeTrace},
		"a receive and a send, in stable failures":   {"counter", "counter", "counter_counter.f", obligationir.IRRefinementModeStableFailure},
		"axioms and assumptions, divergence-freedom": {"map", "countdown", "map_countdown.fd", obligationir.IRRefinementModeFailuresDivergence},
	}
	backEnds := map[target.Name]string{
		target.NameIsabelle: ".thy",
		target.NameLean:     ".lean",
		target.NameIRJSON:   ".json",
	}

	for name, pair := range pairs {
		for backEnd, ext := range backEnds {
			t.Run(name+"/"+string(backEnd), func(t *testing.T) {
				// Setup
				dir := filepath.Join("..", "testdata", "formula")
				spec, err := coreio.Load(filepath.Join(dir, pair.Spec+".puml"))
				if err != nil {
					t.Fatal(err)
				}
				impl, err := coreio.Load(filepath.Join(dir, pair.Impl+".puml"))
				if err != nil {
					t.Fatal(err)
				}
				golden := filepath.Join(dir, "refinement", pair.Golden+ext)
				var sb strings.Builder

				// Execute
				err = target.CompileDiagrams(&sb, pair.Mode, spec, impl, backEnd)

				// Assert
				if err != nil {
					t.Fatal(err)
				}
				if *update {
					if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(golden, []byte(sb.String()), 0o644); err != nil {
						t.Fatal(err)
					}
				}
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
