package target_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
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
