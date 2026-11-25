package csdf

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v4/csdf/clause"
	"github.com/google/go-cmp/cmp"
)

func TestParseEdgeLine(t *testing.T) {
	// Execute
	got, err := ParseEdgeLine("a --> b : HALT?why ; g ; p", 7)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := Edge{Src: "a", Dst: "b", Event: "HALT?why", Line: 7,
		Guard: &ClauseSyntax{Pos: Pos{7, 22}, Text: "g", Syntax: clause.Syntax{Conjuncts: []clause.SynConjunct{&clause.SynAtom{Parts: []clause.SynPart{&clause.SynText{Text: "g"}}}}}},
		Post:  &ClauseSyntax{Pos: Pos{7, 26}, Text: "p", Syntax: clause.Syntax{Conjuncts: []clause.SynConjunct{&clause.SynAtom{Parts: []clause.SynPart{&clause.SynText{Text: "p"}}}}}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseEdgeLine (-want +got):\n%s", diff)
	}
}

func TestParseEdgeLineRefusesWithTheLine(t *testing.T) {
	// Execute
	_, err := ParseEdgeLine("a --> b : HALT??", 7)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "line 7") {
		t.Errorf("ParseEdgeLine: got %v, want an error at line 7", err)
	}
}
