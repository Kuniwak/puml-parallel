package isabelle_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/csdf/obligationir/isabelle"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "rewrite the expected obligations")

func TestWriteFormulaLivelockFree(t *testing.T) {
	testCases := map[string]string{
		"a tau countdown spells its clauses over val":                "countdown",
		"maps, a received field and an ∃ under the axioms they need": "map",
		"a diagram with no tau cycle has no obligation":              "visible",
	}

	for name, file := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			base := filepath.Join("..", "testdata", "formula", file)
			src, err := os.ReadFile(base + ".puml")
			if err != nil {
				t.Fatal(err)
			}
			d, err := coreio.Read(string(src))
			if err != nil {
				t.Fatal(err)
			}
			ir, err := obligationir.BuildFormulaLivelockFree(d)
			if err != nil {
				t.Fatal(err)
			}
			var sb strings.Builder

			// Execute
			err = isabelle.WriteFormulaLivelockFree(&sb, ir)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.WriteFile(base+".thy", []byte(sb.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(base + ".thy")
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), sb.String()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
