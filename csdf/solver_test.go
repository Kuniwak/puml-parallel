package csdf

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSolveJSON(t *testing.T) {
	// Arrange
	group := StateWithID{
		ID: "s0",
		State: State{
			Name: "Initial",
			Vars: []StateVar{{Name: "a"}, {Name: "b"}},
		},
	}

	// Act
	result := SolveJSON(PostSolverInput{
		StateGroup:    group,
		EncodedValues: `[1, {"nested": ["ok"]}]`,
	})

	// Assert
	if result.Kind != PostSolverResultOK {
		t.Fatalf("SolveJSON() kind = %v, want OK; err = %v", result.Kind, result.Err)
	}
	if len(result.State.Values) != 2 || result.State.Values[0].Name != "a" || result.State.Values[1].Name != "b" {
		t.Errorf("SolveJSON() state values = %#v", result.State.Values)
	}
}

func TestSolveJSONReadsNegativeZeroAsZero(t *testing.T) {
	// Setup
	group := StateWithID{ID: "s0", State: State{Name: "Initial", Vars: []StateVar{{Name: "a"}, {Name: "b"}}}}

	// Execute
	result := SolveJSON(PostSolverInput{StateGroup: group, EncodedValues: `[-0.0, [-0, -0e5, -1.5]]`})

	// Assert
	if result.Kind != PostSolverResultOK {
		t.Fatalf("SolveJSON() kind = %v, want OK; err = %v", result.Kind, result.Err)
	}
	want := []StateValue{
		{Name: "a", Value: json.Number("0.0")},
		{Name: "b", Value: []any{json.Number("0"), json.Number("0.0"), json.Number("-1.5")}},
	}
	if diff := cmp.Diff(want, result.State.Values); diff != "" {
		t.Errorf("SolveJSON() mismatch (-want +got):\n%s", diff)
	}
}
