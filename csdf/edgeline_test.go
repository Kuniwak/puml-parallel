package csdf

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseEdgeLine(t *testing.T) {
	// Execute
	got, err := ParseEdgeLine("a --> b : HALT?why ; g ; p", 7, acceptClauses)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := Edge{Src: "a", Dst: "b", Event: "HALT?why", Guard: "g", Post: "p", Line: 7}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ParseEdgeLine (-want +got):\n%s", diff)
	}
}

func TestParseEdgeLineRefusesWithTheLine(t *testing.T) {
	// Execute
	_, err := ParseEdgeLine("a --> b : HALT??", 7, acceptClauses)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "line 7") {
		t.Errorf("ParseEdgeLine: got %v, want an error at line 7", err)
	}
}
