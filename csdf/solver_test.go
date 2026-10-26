package csdf

import (
	"encoding/json"
	"testing"
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
	// Arrange
	group := StateWithID{ID: "s0", State: State{Name: "Initial", Vars: []StateVar{{Name: "a"}, {Name: "b"}}}}

	// Act
	result := SolveJSON(PostSolverInput{StateGroup: group, EncodedValues: `[-0.0, [-0, -0e5, -1.5]]`})

	// Assert
	if result.Kind != PostSolverResultOK {
		t.Fatalf("SolveJSON() kind = %v, want OK; err = %v", result.Kind, result.Err)
	}
	if got := result.State.Values[0].Value; got != json.Number("0.0") {
		t.Errorf("SolveJSON() a = %#v, want 0.0", got)
	}
	want := []any{json.Number("0"), json.Number("0.0"), json.Number("-1.5")}
	got, _ := result.State.Values[1].Value.([]any)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("SolveJSON() b = %#v, want %#v", result.State.Values[1].Value, want)
	}
}
