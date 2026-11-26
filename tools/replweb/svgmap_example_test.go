package replweb

import (
	"os"
	"testing"

	"github.com/Kuniwak/puml-parallel/v5/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v5/csdf/coreio"
	"github.com/google/go-cmp/cmp"
)

// testdata/ATTEMPTS.svg is what PlantUML 1.2026.10 renders
// examples/promote/ATTEMPTS.puml as, in its directory for its !include:
// plantuml -pipe -tsvg < ATTEMPTS.puml. Its local diagram has two parallel
// edges from attNone to attSent, SUBMIT before RESUBMIT; PlantUML writes
// their links in that order.
func TestMapSVGTellsParallelLocalEdgesApartAsPlantUMLDrawsThem(t *testing.T) {
	// Setup
	source, err := os.ReadFile("../../examples/promote/ATTEMPTS.puml")
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := animation.ReadDiagram(source, "../../examples/promote", coreio.FileIncluder("../../examples/promote"))
	if err != nil {
		t.Fatal(err)
	}
	svg, err := os.ReadFile("testdata/ATTEMPTS.svg")
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, err := MapSVG(svg, d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := FamilyHighlights{
		States: map[string]string{"attNone": "ent0003", "attSent": "ent0004", "attStuck": "ent0005", "attDone": "ent0006"},
		Edges:  map[int]string{0: "lnk11", 1: "lnk12", 2: "lnk13", 3: "lnk14", 4: "lnk15"},
		Start:  "lnk10",
		Block:  "ent0002",
	}
	if diff := cmp.Diff(want, got.Families["attempts"]); diff != "" {
		t.Errorf("MapSVG mismatch (-want +got):\n%s", diff)
	}
}
