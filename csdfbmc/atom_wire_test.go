package csdfbmc_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/google/go-cmp/cmp"
)

// An atom as the Go side writes it is what the reference glue reads: a post
// atom parsed from its surface syntax gets its values after the step from
// the glue run as a command.
func TestTheGlueReadsTheAtomsTheToolsWrite(t *testing.T) {
	// Setup
	f, err := clause.Parse("$x' in ($x, $x + 1)")
	if err != nil {
		t.Fatal(err)
	}
	wire, terms := clause.Wire(f.Conjuncts[0].(clause.Atom))

	// Execute
	answers := runGlue(t,
		map[string]any{"op": "hello", "protocol": 2},
		map[string]any{"op": "post", "atom": wire, "values": [][]any{{1}}},
	)

	// Assert
	wantTerms := []clause.Term{clause.Free{Name: "x", Primed: true}, clause.Free{Name: "x"}}
	if diff := cmp.Diff(wantTerms, terms); diff != "" {
		t.Errorf("Wire terms mismatch (-want +got):\n%s", diff)
	}
	want := []map[string]any{
		{"language": "python", "protocol": float64(2), "supports": []any{"symmetric"}},
		{"results": []any{[]any{[]any{float64(1)}, []any{float64(2)}}}},
	}
	if diff := cmp.Diff(want, answers); diff != "" {
		t.Errorf("answers mismatch (-want +got):\n%s", diff)
	}
}
